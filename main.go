package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type AppStatus struct {
	Name       string `json:"name"`
	StatusText string `json:"statusText"`
	Priority   int32  `json:"priority"`
}

type AppList struct {
	Frequency int32       `json:"frequency"`
	Randomise int32       `json:"randomise"`
	Apps      []AppStatus `json:"apps"`
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
	// TODO: deduplicate
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

func manageStatus() {
	for true {
		// get running apps
		// get app to write
		// write app status
		// sleep for <time>
	}
}

func main() {
	// load app config
	// load other config

	apps := parseAppsFromFile()
	fmt.Println(apps)

	splitArr := getRunningApps()
	for i := 0; i < len(splitArr); i++ {
		fmt.Println(splitArr[i][3])
	}
}
