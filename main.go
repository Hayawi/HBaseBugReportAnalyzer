package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

type report struct {
	xml.Name
	Status   string `xml:"channel>item>status"`
	Created  string `xml:"channel>item>created"`
	Resolved string `xml:"channel>item>resolved"`
	Type     string `xml:"channel>item>type"`
}

type statistics struct {
	TypeNames        []string
	States           []string
	CategorizedTotal map[string]int
	Total            int
	BugStates        map[string]int
	BugTimeCount     int
	BugMinTime       time.Duration
	BugMaxTime       time.Duration
	BugMedianTime    time.Duration
	BugAverageTime   time.Duration
	BugTimes         []time.Duration
}

func (s *statistics) updateBugTimes(duration float64) {
	durString := strconv.FormatFloat(duration, 'g', 16, 64)
	durTime, err := time.ParseDuration(durString + "s")
	check(err)
	if duration < s.BugMinTime.Seconds() {
		s.BugMinTime = durTime
	}
	if duration > s.BugMaxTime.Seconds() {
		s.BugMaxTime = durTime
	}

	avgString := strconv.FormatFloat((s.BugAverageTime.Seconds()*float64(s.BugTimeCount)+duration)/float64(s.BugTimeCount+1), 'g', 16, 64)
	s.BugAverageTime, err = time.ParseDuration(avgString + "s")
	check(err)
	s.BugTimeCount++

	s.BugTimes = append(s.BugTimes, durTime)
}

func (s *statistics) sortBugTimes() {
	sort.Slice(s.BugTimes, func(i, j int) bool { return s.BugTimes[i] < s.BugTimes[j] })
}

var lock = sync.RWMutex{}

func check(e error) {
	if e != nil && e.Error() != "EOF" {
		log.Panicln(e)
	}
}

func fileReader(fileToOpen string) ([]byte, error) {
	_, err := fmt.Println("Opening", fileToOpen)
	check(err)
	fileToRead, err := os.Open(fileToOpen)
	check(err)
	defer fileToRead.Close()
	fileStat, err := fileToRead.Stat()
	check(err)
	data := make([]byte, fileStat.Size())
	_, err = io.ReadFull(fileToRead, data)
	return data, err
}

func extractData(data []byte) (*report, error) {
	result := &report{}
	err := xml.Unmarshal(data, result)
	check(err)
	return result, err
}

func addData(res *report, typeNames *[]string, states *[]string) {
	foundType := false
	foundStatus := false
	for _, types := range *typeNames {
		if types == res.Type {
			foundType = true
		}
	}
	for _, status := range *states {
		if status == res.Status {
			foundStatus = true
		}
	}
	if !foundType {
		*typeNames = append(*typeNames, res.Type)
	}
	if !foundStatus {
		*states = append(*states, res.Status)
	}
}

func writeToMap(totalMap *map[string]int, statusName string) {
	lock.Lock()
	defer lock.Unlock()
	(*totalMap)[statusName]++
}

func (s *statistics) initializeStatistics() {
	typeNames := []string{}
	states := []string{}
	categorizedTotal := map[string]int{}
	total := 0
	bugStates := map[string]int{}
	s.TypeNames = typeNames
	s.States = states
	s.CategorizedTotal = categorizedTotal
	s.Total = total
	s.BugStates = bugStates
	s.BugMinTime = math.MaxInt64
	s.BugMaxTime = math.MinInt64
	s.BugMedianTime = 0
	s.BugAverageTime = 0
}

func main() {
	directoryName := "C:\\Users\\Hayawi\\Downloads\\hbaseBugReport\\"
	directory, _ := ioutil.ReadDir(directoryName)
	ch := make(chan string, 100)
	output := statistics{}
	output.initializeStatistics()
	task := func(file string, output *statistics) {
		ch <- "Executing"
		data, err := fileReader(file)
		check(err)
		res, err := extractData(data)
		check(err)
		addData(res, &output.TypeNames, &output.States)
		if res.Type == "Bug" {
			writeToMap(&output.BugStates, res.Status)
			if res.Status == "Closed" || res.Status == "Resolved" {
				setDurations(output, res.Created, res.Resolved)
			}
		}
		writeToMap(&output.CategorizedTotal, res.Type)
		output.Total++
	}

	for _, file := range directory {
		go task(directoryName+file.Name(), &output)
	}

	for range directory {
		<-ch
	}

	for output.Total < len(directory) {
		time.Sleep(5)
	}

	output.sortBugTimes()
	output.BugMedianTime = output.BugTimes[output.BugTimeCount/2]

	fmt.Println("Bug Min Time:", output.BugMinTime.String())
	fmt.Println("Bug Max Time:", output.BugMaxTime.String())
	fmt.Println("Bug Median Time:", output.BugMedianTime.String())
	fmt.Println("Bug Average Time:", output.BugAverageTime.String())
	fmt.Println("Total Number of Reports:", output.Total)
	for _, types := range output.TypeNames {
		fmt.Println("Total Number of", types, ":", output.CategorizedTotal[types])
	}
	for _, status := range output.States {
		fmt.Println("Total Number of Bugs With Status", status, ":", output.BugStates[status])
	}
}

func setDurations(output *statistics, createdDate, resolvedDate string) float64 {

	if string(createdDate[6]) == " " {
		createdDate = createdDate[0:5] + "0" + createdDate[5:]
	}
	if string(resolvedDate[6]) == " " {
		resolvedDate = resolvedDate[0:5] + "0" + resolvedDate[5:]
	}

	createdTime, err := time.Parse(time.RFC1123Z, createdDate)
	check(err)
	resolvedTime, err := time.Parse(time.RFC1123Z, resolvedDate)
	check(err)

	dur := resolvedTime.Sub(createdTime)

	output.updateBugTimes(dur.Seconds())

	return dur.Seconds()
}
