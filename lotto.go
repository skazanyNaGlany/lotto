package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/thoas/go-funk"
)

const REPO_URL = "http://www.mbnet.com.pl/dl.txt"
const RESULT_FILE_FORMAT = "%s-dl.txt"

var RESULTS_PATHNAME, _ = filepath.Abs(filepath.Dir(os.Args[0]))
var RESULTS_FULL_PATHNAME = RESULTS_PATHNAME + "/results"
var RESULT_FORMAT_RE = regexp.MustCompile(`^(?P<seqNo>\d+)\. (?P<day>\d+)\.(?P<month>\d+)\.(?P<year>\d+)\ (?P<n0>\d+)\,(?P<n1>\d+)\,(?P<n2>\d+)\,(?P<n3>\d+)\,(?P<n4>\d+)\,(?P<n5>\d+)$`)
var CURRENT_LOCATION = time.Now().Location()

type ResultEntry struct {
	seqNo    int
	dateTime time.Time
	numbers  [6]int
}

func downloadLastResultsFile() string {
	now := time.Now()
	formattedTime := fmt.Sprintf("%d-%02d-%02d",
		now.Year(), now.Month(), now.Day())

	latestResultPathname := RESULTS_FULL_PATHNAME + "/" + fmt.Sprintf(RESULT_FILE_FORMAT, formattedTime)

	log.Println("Looking for", latestResultPathname)

	exists := true

	_, err := os.Stat(latestResultPathname)

	if err != nil {
		exists = false
	}

	if exists {
		log.Println("Latest file with results already exists in", latestResultPathname)

		return latestResultPathname
	}

	log.Println("Latest file with results does not exists, downloading", REPO_URL, "to", latestResultPathname)

	response, err := http.Get(REPO_URL)

	if err != nil {
		log.Panicln("err", err)
	}

	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Panicln("err", err)
	}

	err = os.WriteFile(latestResultPathname, resBody, 0666)

	if err != nil {
		log.Panicln("err", err)
	}

	return latestResultPathname
}

func findNamedMatches(regex *regexp.Regexp, str string) map[string]string {
	match := regex.FindStringSubmatch(str)
	results := map[string]string{}

	if match == nil {
		return results
	}

	for i, name := range match {
		if i == 0 {
			// skip 0 match, since it will be whole line
			continue
		}

		results[regex.SubexpNames()[i]] = name
	}

	return results
}

func parseResultsFile(pathname string) ([]ResultEntry, error) {
	bytes, err := os.ReadFile(pathname)

	if err != nil {
		return nil, err
	}

	parsedResults := make([]ResultEntry, 0)
	resultsStr := string(bytes)

	for _, line := range strings.Split(resultsStr, "\n") {
		line := strings.TrimSpace(line)

		if line == "" {
			continue
		}

		parsedLineRe := findNamedMatches(RESULT_FORMAT_RE, line)

		if len(parsedLineRe) < 10 {
			continue
		}

		seqNo, _ := strconv.ParseInt(parsedLineRe["seqNo"], 10, 32)
		day, _ := strconv.ParseInt(parsedLineRe["day"], 10, 32)
		month, _ := strconv.ParseInt(parsedLineRe["month"], 10, 32)
		year, _ := strconv.ParseInt(parsedLineRe["year"], 10, 32)

		n0, _ := strconv.ParseInt(parsedLineRe["n0"], 10, 32)
		n1, _ := strconv.ParseInt(parsedLineRe["n1"], 10, 32)
		n2, _ := strconv.ParseInt(parsedLineRe["n2"], 10, 32)
		n3, _ := strconv.ParseInt(parsedLineRe["n3"], 10, 32)
		n4, _ := strconv.ParseInt(parsedLineRe["n4"], 10, 32)
		n5, _ := strconv.ParseInt(parsedLineRe["n5"], 10, 32)

		entry := ResultEntry{
			seqNo:    int(seqNo),
			dateTime: time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, CURRENT_LOCATION),
			numbers:  [6]int{int(n0), int(n1), int(n2), int(n3), int(n4), int(n5)},
		}

		parsedResults = append(parsedResults, entry)
	}

	return parsedResults, nil
}

func sortResultEntrySlice(s []ResultEntry) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].seqNo < s[j].seqNo
	})
}

func findResultIndex(date time.Time, results []ResultEntry) int {
	for i, iresult := range results {
		if iresult.dateTime.Equal(date) {
			return i
		}
	}

	return -1
}

func getNumbersStatistics(
	startDate time.Time,
	endDate time.Time,
	minNumber int,
	maxNumber int,
	results []ResultEntry) map[int]int {
	stats := make(map[int]int)

	startIndex := findResultIndex(startDate, results)

	if startIndex == -1 {
		log.Println("cannot find result from day", startDate.String())
		return nil
	}

	endIndex := findResultIndex(endDate, results)

	if endIndex == -1 {
		log.Println("cannot find result from day", endDate.String())
		return nil
	}

	for i := minNumber; i < maxNumber+1; i++ {
		stats[i] = 0
	}

	for i := startIndex; i < endIndex+1; i++ {
		iresult := results[i]

		for _, num := range iresult.numbers {
			stats[num]++
		}
	}

	return stats
}

func sortStats(stats map[int]int) *orderedmap.OrderedMap[int, int] {
	values := funk.Values(stats).([]int)

	sort.Slice(values, func(i, j int) bool {
		return values[i] > values[j]
	})

	newStats := orderedmap.NewOrderedMap[int, int]()

	for _, ivalue := range values {
		for mapKey, mapValue := range stats {
			if mapValue == ivalue {
				newStats.Set(mapKey, mapValue)

				delete(stats, mapKey)
				break
			}
		}
	}

	return newStats
}

func printStats(sortedStats *orderedmap.OrderedMap[int, int]) {
	maxKey := 0
	maxValue := 0

	for el := sortedStats.Front(); el != nil; el = el.Next() {
		if maxKey < el.Key {
			maxKey = el.Key
		}

		if maxValue < el.Value {
			maxValue = el.Value
		}
	}

	keyWidth := len(fmt.Sprint(maxKey))
	valueWidth := len(fmt.Sprint(maxValue))

	// it will be something like:
	// %6d   [%6dx]      %v\n
	// with parameters:
	// key, value, stars
	format := ""
	format += "%" + fmt.Sprint(keyWidth) + "d"
	format += "   "
	format += "%" + fmt.Sprint(valueWidth) + "dx"
	format += "      %v\n"

	for el := sortedStats.Front(); el != nil; el = el.Next() {
		countStr := strings.Repeat("*", el.Value)

		fmt.Printf(format, el.Key, el.Value, countStr)
	}
}

func main() {
	log.Println("Repo URL", REPO_URL)
	log.Println("Saving downloaded results to", RESULTS_FULL_PATHNAME)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	os.MkdirAll(RESULTS_FULL_PATHNAME, 0777)

	pathname := downloadLastResultsFile()

	if pathname == "" {
		log.Panicln("cannot find file with the latest results")
	}

	parsed, err := parseResultsFile(pathname)

	if err != nil {
		log.Panicln("err", err)
	}

	sortResultEntrySlice(parsed)

	endDate := parsed[len(parsed)-1].dateTime
	startDate := endDate.AddDate(0, 0, -367)
	// startDate := endDate.AddDate(0, 0, -7)
	// startDate := time.Now().AddDate(0, 0, -7)

	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, CURRENT_LOCATION)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, CURRENT_LOCATION)

	log.Println("Start date", startDate)
	log.Println("End date", endDate)

	stats := getNumbersStatistics(
		startDate,
		endDate,
		1,
		49,
		parsed)

	sortedStats := sortStats(stats)

	log.Println("Sorted results:")
	printStats(sortedStats)
}
