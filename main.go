package main

import (
	"bytes"
	"context"
	"darts/gql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Khan/genqlient/graphql"
)

type AppStatus struct {
	Name       string `json:"name"`
	StatusText string `json:"statusText"`
	Priority   int32  `json:"priority"`
}

type AppList struct {
	Frequency      int32       `json:"frequency"`
	Randomise      int32       `json:"randomise"`
	FallbackStatus string      `json:"fallback"`
	Apps           []AppStatus `json:"apps"`
}

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

var graphqlClient graphql.Client

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
	for i := 1; i < len(splitStr); i++ {
		splitArr = append(splitArr, splitToArray(splitStr[i]))
	}
	splitArr = filterApps(splitArr, lenCheckArr)
	return splitArr
}

func getRunningApps() [][]string {
	a := exec.Command("ps", "-A")
	var out bytes.Buffer
	a.Stdout = &out
	a.Run()
	splitArr := getRelevantArr(string(out.Bytes()))
	return splitArr
}

func toSingletonArray(arr [][]string) (retArr []string) {
	for _, strArr := range arr {
		retArr = append(retArr, strArr[3])
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

func writeToGithubStatus(str string, expiry time.Time) bool {
	fmt.Printf("Writing to Github status: %s\n", str)
	gqlReq := gql.ChangeUserStatusInput{ClientMutationId: "darts/status", Message: str}
	res, err := gql.UpdateStatus(context.Background(), graphqlClient, gqlReq)
	fmt.Println(res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Call to Github failed: %s\n", err)
	}
	return err == nil
}

func getAppMap(sortedArr []AppStatus) map[int32][]AppStatus {
	retMap := map[int32][]AppStatus{}
	for _, app := range sortedArr {
		if retMap[app.Priority] == nil {
			retMap[app.Priority] = []AppStatus{app}
		} else {
			retMap[app.Priority] = append(retMap[app.Priority], app)
		}
	}
	return retMap
}

func getCurrentApp(activeApps map[string]bool, appList []AppStatus, appMap map[int32][]AppStatus, fallback string) string {
	for _, app := range appList {
		appIsRunning := activeApps[app.Name]
		if appIsRunning {
			return app.StatusText
		}
	}
	return fallback
}

func manageStatus() {
	appConfig := parseAppsFromFile()
	appList := appConfig.Apps
	sort.Slice(appList, func(i, j int) bool { return appList[i].Priority < appList[j].Priority })
	appMap := getAppMap(appList)
	for true {
		activeApps := toHashSet(toSingletonArray(getRunningApps()))
		curStr := getCurrentApp(activeApps, appList, appMap, appConfig.FallbackStatus)
		expiry := time.Now().Add(time.Duration(appConfig.Frequency) * time.Second * 3) // expire after 3 missed updates
		writeToGithubStatus(curStr, expiry)
		time.Sleep(time.Duration(appConfig.Frequency) * time.Second)
	}
}

func resetOnClose() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig // buffered channels block if empty
	writeToGithubStatus("", time.Now())
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
	resp, err := gql.MyQuery(context.Background(), graphqlClient)
	fmt.Println(resp, err)
}

func main() {
	go resetOnClose()

	initClient()
	manageStatus()
}
