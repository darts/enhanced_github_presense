package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	graphql "github.com/hasura/go-graphql-client"
)

type AppStatus struct {
	Name       string `json:"name"`
	StatusText string `json:"statusText"`
	Priority   int32  `json:"priority"`
	Emoji      string `json:"emoji"`
}

type AppList struct {
	Frequency      int32       `json:"frequency"`
	FallbackStatus string      `json:"fallback"`
	FallbackEmoji  string      `json:"fallback_emoji"`
	Apps           []AppStatus `json:"apps"`
}

type ChangeUserStatusInput struct {
	ClientMutationId string `json:"clientMutationId"`
	// The emoji to represent your status. Can either be a native Unicode emoji or an emoji name with colons, e.g., :grinning:.
	Emoji     string    `json:"emoji"`
	ExpiresAt time.Time `json:"expiresAt"`
	Message   string    `json:"message"`
}

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

var graphqlClient *graphql.Client
var isWindows bool

var statusMutation struct {
	ChangeUserStatus struct {
		ClientMutationId string
		Status           struct {
			Message string
		}
	} `graphql:"changeUserStatus(input: $input)"`
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "bearer "+t.key)
	return t.wrapped.RoundTrip(req)
}

func filterApps[K any](ss []K, test func(K) bool) (ret []K) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

// TODO: make generic after looking up go type system
func lenCheck(item string) (ret bool) {
	ret = len(item) > 0
	return
}
func lenCheckArr(item []string) (ret bool) {
	ret = len(item) > 0
	return
}

func splitToArray(s string) []string {
	splitStrs := strings.Split(s, " ")
	ret := filterApps(splitStrs, lenCheck)
	return ret
}

func getRelevantArr(rawStr string) (splitArr [][]string) {
	splitStr := strings.Split(rawStr, "\n")
	i := 1
	if isWindows {
		i = 3
	}
	for ; i < len(splitStr); i++ {
		if len(splitStr[i]) > 3 {
			splitArr = append(splitArr, splitToArray(splitStr[i]))
		}
	}
	splitArr = filterApps(splitArr, lenCheckArr)
	return splitArr
}

func getAppsWin() (splitArr [][]string) {
	a := exec.Command("powershell", "-NoProfile", "Get-Process | Where-Object {$_.MainWindowHandle -ne 0} | Select-Object ProcessName")
	var out bytes.Buffer
	a.Stdout = &out
	a.Run()
	splitArr = getRelevantArr(out.String())
	return
}

func getAppsLinux() (splitArr [][]string) {
	a := exec.Command("ps", "-A")
	var out bytes.Buffer
	a.Stdout = &out
	a.Run()
	splitArr = getRelevantArr(out.String())
	return
}

func getRunningApps() [][]string {
	if isWindows {
		return getAppsWin()
	}
	return getAppsLinux()
}

func toSingletonArray(arr [][]string) (retArr []string) {
	idx := 3
	if isWindows {
		idx = 0
	}
	for _, strArr := range arr {
		retArr = append(retArr, strings.ToLower(strArr[idx]))
	}
	return
}

func toHashSet(arr []string) map[string]bool {
	retMap := map[string]bool{}
	for _, strArr := range arr {
		retMap[strArr] = true
	}
	return retMap
}

func parseAppsFromFile() *AppList {
	jsonFilename := "./appsStatus.json"
	jsonFile, err := os.ReadFile(jsonFilename)
	if err != nil {
		log.Fatalf("Opening apps list file failed: %v\n", err)
	}
	var apps AppList
	err = json.Unmarshal(jsonFile, &apps)
	if err != nil {
		log.Fatalf("Parsing apps list failed: %v\n", err)
	}
	fmt.Println("Loaded JSON from file.")
	return &apps
}

func writeToGithubStatus(str string, emoji string, expiry time.Time) bool {
	fmt.Printf("Writing to Github status: %s\n", str)
	gqlReq := map[string]interface{}{
		"input": ChangeUserStatusInput{ClientMutationId: "darts/status", Message: str, Emoji: emoji, ExpiresAt: expiry},
	}
	err := graphqlClient.Mutate(context.Background(), &statusMutation, gqlReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Call to Github failed: %s\n", err)
	}
	return err == nil
}

func getCurrentApp(activeApps map[string]bool, appList []AppStatus, config AppList) (string, string) {
	allListedRunningApps := []AppStatus{}
	minPriority := int32(math.Inf(1)) - 1
	hasActiveApp := false
	for _, app := range appList {
		if activeApps[app.Name] {
			hasActiveApp = true
			if app.Priority < minPriority {
				minPriority = app.Priority
				allListedRunningApps = []AppStatus{app}
			} else if app.Priority == minPriority {
				allListedRunningApps = append(allListedRunningApps, app)
			}
		}
	}
	if !hasActiveApp {
		return config.FallbackStatus, config.FallbackEmoji
	}

	aRandomApp := allListedRunningApps[rand.Intn(len(allListedRunningApps))]

	return aRandomApp.StatusText, aRandomApp.Emoji
}

func manageStatus() {
	appConfig := parseAppsFromFile()
	appList := appConfig.Apps
	sort.Slice(appList, func(i, j int) bool { return appList[i].Priority < appList[j].Priority })
	for {
		activeApps := toHashSet(toSingletonArray(getRunningApps()))
		curStr, curEmoji := getCurrentApp(activeApps, appList, *appConfig)
		expiry := time.Now().Add(time.Duration(appConfig.Frequency) * time.Second * 3) // expire after 3 missed updates
		writeToGithubStatus(curStr, curEmoji, expiry)
		time.Sleep(time.Duration(appConfig.Frequency) * time.Second)
	}
}

func resetOnClose() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig // buffered channels block if empty
	writeToGithubStatus("", "", time.Now())
	os.Exit(0)
}

func initClient() {
	authToken := os.Getenv("GITHUB_PAT")

	if authToken == "" {
		log.Fatal("GITHUB_PAT not set")
	}

	httpClient := http.Client{
		Transport: &authedTransport{
			key:     authToken,
			wrapped: http.DefaultTransport,
		},
	}

	graphqlClient = graphql.NewClient("https://api.github.com/graphql", &httpClient)

	rand.Seed(time.Now().Unix())
	isWindows = runtime.GOOS == "windows"
}

func main() {
	go resetOnClose()
	initClient()
	manageStatus()
}
