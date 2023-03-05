package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
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
		// TODO: replace with log.fatal
		fmt.Fprintf(os.Stderr, "Opening apps list file failed: %v\n", err)
		os.Exit(1)
	}
	var apps AppList
	err = json.Unmarshal(jsonFile, &apps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing apps list failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "Loaded JSON from file.")
	return &apps
}

func writeToGithubStatus(str string) bool {
	fmt.Printf("Writing to Github status: %s\n", str)
	fmt.Println("UPDATE GRAPHQL QUERY")
	return true
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

func manageStatus() {
	appConfig := parseAppsFromFile()
	appList := appConfig.Apps
	sort.Slice(appList, func(i, j int) bool { return appList[i].Priority > appList[j].Priority })
	appMap := getAppMap(appList)
	fmt.Println(appMap)
	for true {
		time.Sleep(time.Duration(appConfig.Frequency) * time.Millisecond)
		// get running apps
		// get app to write
		// write app status
		// sleep for <time>
	}
}

func main() {
	appConfig := parseAppsFromFile()
	fmt.Println(appConfig)

	arr := toHashSet(toSingletonArray(getRunningApps()))
	fmt.Println(arr)

	manageStatus()
}
