package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type comicData struct {
	Month json.Number `json:"month"`
	Num int `json:"num"`
	Link string `json:"link"`
	Year json.Number `json:"year"`
	News string `json:"news"`
	SafeTitle string `json:"safe_title"`
	Transcript string `json:"transcript"`
	Alt string `json:"alt"`
	Image string `json:"img"`
	Title string `json:"title"`
	Day json.Number `json:"day"`
}

var comicsData []comicData = make([]comicData, numComics)
var numComics = 2428 

var monthIndex []int = make([]int, numComics)
var numIndex []int = make([]int, numComics) // kind of pointless because comicsData is already sorted by this key
var yearIndex []int = make([]int, numComics) // kind of pointless because comicsData is already sorted by this key
var titleIndex []int = make([]int, numComics)

// Writes to stdin and awaits parameters from the user
// Prints out the struct containing the json data for comics satisfying the user's criteria
func main() {
	flag := os.Args[1]
	switch flag {
	case "build":
		build()
	case "search":
		awaitInput()
	}
}

func awaitInput() {
	// fill comicsData with json data from file
	loadCompressedComicData("compressed_xkcd_json")

	// load the "indexes"
	// we can search these indices and return the corresponding json data
	loadIndex("month", monthIndex)
	loadIndex("num", numIndex)
	loadIndex("year", yearIndex)
	loadIndex("title", titleIndex)

	// listen to stdin for filtering parameters, then print
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Please query with the following format: [key] [operand] [value]")
	fmt.Println("The acceptable keys are: month, num, year, title")
	fmt.Println("The acceptable operands are: >=, <=, =")
	fmt.Println("The value can be any string, NOT wrapped in quotation marks")
	fmt.Println("Example: title = 1/10,000th Scale World")
	fmt.Print("> ")
	for scanner.Scan() {
		args := strings.Fields(scanner.Text())
		key := args[0]
		inequality := args[1]
		
		// get value
		var sb strings.Builder
		for i, word := range args[2:] {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(word) 
		}
		value := sb.String()
		
		// search the index and print the corresponding comics' information
		var indexToUse []int
		switch key {
		case "month":
			indexToUse = monthIndex
		case "num":
			indexToUse = numIndex
		case "year":
			indexToUse = yearIndex
		case "title":
			indexToUse = titleIndex
		default:
			fmt.Println("Error: invalid key specified.")
			fmt.Print("> ")
			continue
		}
		
		// get index bounds of comics to print [leftBound, rightBound)
		leftBound := 0 
		rightBound := numComics
		switch inequality {
		case "<=":
			rightBound = findIndex(indexToUse, key, inequality, value, false)
		case ">=":
			leftBound = findIndex(indexToUse, key, inequality, value, true)
		case "=":
			rightBound = findIndex(indexToUse, key, inequality, value, false)
			leftBound = findIndex(indexToUse, key, inequality, value, true)
		default:
			fmt.Println("Error: invalid operand specified.")
			fmt.Print("> ")
			continue
		}
		
		// print the data of the comics!
		for i := leftBound; i < rightBound; i++ {
			fmt.Println(comicsData[indexToUse[i]])
		}
		if leftBound >= rightBound {
			fmt.Println("No comics satisfy your search criteria")
		}

		fmt.Print("> ")
	}
}

// creates the files that act as databases, holding the comicsData and indices
// I realize this could be much more efficient, but will leave it for some time in the future
func build() {
	// create file containing json data
	downloadComicsData("xkcd_json")
	// compress file
	compressFile("xkcd_json", "compressed_xkcd_json")
	// load data into slice
	loadCompressedComicData("compressed_xkcd_json")
	// build indices
	createIndex("month")
	loadCompressedComicData("compressed_xkcd_json") // reset slice
	createIndex("num")
	loadCompressedComicData("compressed_xkcd_json") // reset slice
	createIndex("year")
	loadCompressedComicData("compressed_xkcd_json") // reset slice
	createIndex("title")
	loadCompressedComicData("compressed_xkcd_json") // reset slice

}

// returns an index's index specifying the left or right bound to be printed to the user
func findIndex(indexToUse []int, key string, inequality string, value string, isLeftBound bool) int {
	var index int

	switch key {
	case "month":
		if isLeftBound {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Month >= json.Number(value) })
		} else {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Month > json.Number(value) })
		}
		
	case "num":
		value, err := strconv.Atoi(value)
		if err != nil {
			panic(err)
		}
		if isLeftBound {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Num >= value })
		} else {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Num > value })
		}
		
	case "year":
		if isLeftBound {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Year >= json.Number(value) })
		} else {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Year > json.Number(value) })
		}
		
	case "title":
		if isLeftBound {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Title >= value })
		} else {
			index = sort.Search(numComics, func(i int) bool { return comicsData[indexToUse[i]].Title > value })
		}
		
	}
	return index
}

// Writes to a file a list of indices that defines a comicData ordering based on the sortBy key
func createIndex(sortBy string) {
	// initialize slice of "pointers" (we use an index here to do it)
	toSort := make([]int, numComics)
	for i := 0; i < numComics; i++ {
		toSort[i] = i
	}

	// sort by the parameter passed
	coupledSlice := coupledSlice{}
	coupledSlice.SortBy = comicsData
	coupledSlice.ToSort = toSort

	switch sortBy {
	case "month":
		sort.Sort(byMonth(coupledSlice))
	case "num":
		sort.Sort(byNum(coupledSlice))
	case "year":
		sort.Sort(byYear(coupledSlice))
	case "title":
		sort.Sort(byTitle(coupledSlice))
	}

	// write the index to a file
	fileName := sortBy + "_index"
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	for _, location := range coupledSlice.ToSort {
		fmt.Fprintf(file, "%d\n", location)
	} 
}

// reads file containing data for the index specified by the key argument
func loadIndex(key string, index []int) {
	fileName := key + "_index"
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)

	i := 0
	for scanner.Scan() {
		location, err := strconv.Atoi(scanner.Text())
		if err != nil {
			panic(err)
		}
		index[i] = location
		i++
	}
}

// reads from a compressed file and fills the comicsData slice
func loadCompressedComicData(fileName string) {
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	zipReader, err := gzip.NewReader(file)
	defer zipReader.Close()
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(zipReader)
	var currComicData comicData
	for i := 0; i < numComics; i++ {
		scanner.Scan()
		json.Unmarshal(scanner.Bytes(), &currComicData)
		comicsData[i] = currComicData
	}
}

// Query and write xkcd json data to a file
func downloadComicsData(fileName string) {
 	jsonFile, err := os.Create(fileName)
 	defer jsonFile.Close()
 	if err != nil {
 		panic(err)
 	}
 	encoder := json.NewEncoder(jsonFile)

	urlPrefix := "https://xkcd.com/"
	urlSuffix := "/info.0.json"

	// query and write each comic's json data
	for i := 1; i <= numComics; i++ {
		currUrl := urlPrefix + strconv.Itoa(i) + urlSuffix
		var dataToWrite comicData
		if i == 404 {
			// use empty struct for 404 since they "didn't publish anything"
			dataToWrite = comicData{}
		} else {
			dataToWrite = fetch(currUrl)
		}
		err := encoder.Encode(dataToWrite)
		if err != nil {
			panic(err)
		}
	}
}

// Queries a single http endpoint and converts the json data to a comicData struct
func fetch(url string) comicData {
	// get http response
	resp, err := http.Get(url) 
	defer resp.Body.Close()
	if err != nil {
		panic(err)
	}

	// put json data into struct
	var currData comicData
	if err := json.NewDecoder(resp.Body).Decode(&currData); err != nil {
		fmt.Println(url)
		panic(err)
	}
	return currData
}

// I realize I could have compressed directly from the http requests, but am leaving it for the future
func compressFile(fileName string, compressedFileName string) {
	// get data from file to compress
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	reader := bufio.NewReader(file)
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		panic(err)
	}

	// compress and write to new file name
	newFile, err := os.Create(compressedFileName)
	writer := gzip.NewWriter(newFile)
	writer.Write(data)
	writer.Close()
}

// Reads uncompressed xkcd json data from file
func readXkcdData(comicDataBuffer []comicData) {
	// Open the file and load the object back!
    file, err := os.Open("xkcd_json")
    defer file.Close()
    if err != nil {
    	panic(err)
    }

    decoder := json.NewDecoder(file)
    var data comicData
    i := 0
    for {
    	err = decoder.Decode(&data) // get next xkcd struct
	    if err == io.EOF {
	    	break
	    } else if err != nil {
	        panic(err)
	    }
	    comicDataBuffer[i] = data
	    i += 1
    }
    
}


// Everything below is defining types/satisfying interfaces to allow a slice to be sorted based off of another slice

// struct made to help sort toSort based off of sortBy
type coupledSlice struct {
	SortBy []comicData
	ToSort []int
}

type byMonth coupledSlice
type byNum coupledSlice
type byYear coupledSlice
type byTitle coupledSlice

func (slices byMonth) Len() int {
	return len(slices.SortBy)
}

func (slices byNum) Len() int {
	return len(slices.SortBy)
}

func (slices byYear) Len() int {
	return len(slices.SortBy)
}

func (slices byTitle) Len() int {
	return len(slices.SortBy)
}

func (slices byMonth) Less(i, j int) bool {
	if slices.SortBy[i].Month == slices.SortBy[j].Month {
		return slices.SortBy[i].Num < slices.SortBy[j].Num
	}
	return slices.SortBy[i].Month < slices.SortBy[j].Month
}

func (slices byNum) Less(i, j int) bool {
	return slices.SortBy[i].Num < slices.SortBy[j].Num
}

func (slices byYear) Less(i, j int) bool {
	if slices.SortBy[i].Year == slices.SortBy[j].Year {
		return slices.SortBy[i].Num < slices.SortBy[j].Num
	}
	return slices.SortBy[i].Year < slices.SortBy[j].Year
}

func (slices byTitle) Less(i, j int) bool {
	return slices.SortBy[i].Title < slices.SortBy[j].Title 
}

func (slices byMonth) Swap(i, j int) {
	slices.SortBy[i], slices.SortBy[j] = slices.SortBy[j], slices.SortBy[i]
	slices.ToSort[i], slices.ToSort[j] = slices.ToSort[j], slices.ToSort[i]
}

func (slices byNum) Swap(i, j int) {
	slices.SortBy[i], slices.SortBy[j] = slices.SortBy[j], slices.SortBy[i]
	slices.ToSort[i], slices.ToSort[j] = slices.ToSort[j], slices.ToSort[i]
}

func (slices byYear) Swap(i, j int) {
	slices.SortBy[i], slices.SortBy[j] = slices.SortBy[j], slices.SortBy[i]
	slices.ToSort[i], slices.ToSort[j] = slices.ToSort[j], slices.ToSort[i]
}

func (slices byTitle) Swap(i, j int) {
	slices.SortBy[i], slices.SortBy[j] = slices.SortBy[j], slices.SortBy[i]
	slices.ToSort[i], slices.ToSort[j] = slices.ToSort[j], slices.ToSort[i]
}