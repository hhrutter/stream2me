package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var wg sync.WaitGroup

func errorExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

// appendFile appends srcFile to destFile.
func appendFile(srcFileName, destFileName string) error {

	// if file does not exist, create file
	f, err := os.OpenFile(destFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer func() {
		f.Close()
	}()

	buf, err := ioutil.ReadFile(srcFileName)
	if err != nil {
		return err
	}

	_, err = f.Write(buf)

	return err
}

func writeConcatenatedFile(outDir, outFileName, formatStr string, n int) error {

	fmt.Printf("writing %s...\n", outFileName)
	for i := 0; i < n; i++ {
		chunkFileName := filepath.Join(outDir, fmt.Sprintf(formatStr, i))
		err := appendFile(chunkFileName, outFileName)
		if err != nil {
			errorExit(err)
		}
	}

	return nil
}

// download url to fileName.
func download(url, fileName string) (int, error) {

	fmt.Print(".")
	//fmt.Printf("downloading: %s\n", fileName)

	//fmt.Printf("downloading: %s\n", url)
	//fmt.Printf("         to: %s\n", fileName)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("we need to abort => received http error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	err = ioutil.WriteFile(fileName, body, os.ModePerm)

	return len(body), err
}

func urlAndFilePath(baseURL, outDir, fileName string) (url, filePath string) {
	url = baseURL + "/" + fileName
	filePath = filepath.Join(outDir, fileName)
	return
}

func downloadChunk(baseURL, outDir, formatStr string, i int) (int, error) {

	//fmt.Printf("downloadChunk %d\n", i)

	url, filePath := urlAndFilePath(baseURL, outDir, fmt.Sprintf(formatStr, i))

	n, err := download(url, filePath)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func downloadChunks(baseURL, outDir, formatStr string, startInd, count int, done chan bool) error {

	//fmt.Printf("downloadChunks %d startingAt %d\n", count, startInd)

	defer wg.Done()

	for i := startInd; i < startInd+count; i++ {

		url, filePath := urlAndFilePath(baseURL, outDir, fmt.Sprintf(formatStr, i))

		n, err := download(url, filePath)
		if err != nil {
			fmt.Println(err)
			done <- true
		}

		if n == 0 {
			fmt.Printf("Unknown chunk %d", i)
			done <- true
		}

	}

	return nil
}

func downloadStream(baseURL, outDir, formatStr string) (chunks int, err error) {

	i := 0

	for ; ; i++ {

		n, err := downloadChunk(baseURL, outDir, formatStr, i)
		if err != nil {
			return 0, err
		}

		fmt.Print(".")
		if i > 0 {
			if i%100 == 0 {
				fmt.Printf(" %d\n", i)
			} else if i%10 == 0 {
				fmt.Print("\n")
			}
		}

		if n == 0 {
			break
		}

	}

	return i, nil
}

func downloadStreamOptimized(baseURL, outDir, formatStr string, done chan bool) (chunks int, err error) {

	i := 0
	step := 100

	for {

		n, err := downloadChunk(baseURL, outDir, formatStr, i+step-1)
		if err != nil {
			return 0, err
		}

		if n > 0 {
			wg.Add(1)
			go downloadChunks(baseURL, outDir, formatStr, i, step-1, done)
			i += step
			//fmt.Printf("i=%d\n", i)
			continue
		}

		if step > 2 {
			step /= 2
			//fmt.Printf("step=%d\n", step)
			continue
		}

		if step == 2 {
			n, err = downloadChunk(baseURL, outDir, formatStr, i)
			if err != nil {
				return 0, err
			}
		}

		if n > 0 {
			return i + 1, nil
		} else {
			return i, nil
		}

	}

}

func main() {

	// Process command line arguments.
	if len(os.Args) != 3 {
		errorExit(errors.New("Usage: stream2me outFile baseUrl"))
	}

	baseURL := os.Args[2]
	fmt.Printf("baseURL: %s\n", baseURL)

	outFileName := os.Args[1]
	fmt.Printf("outFile: %s\n", outFileName)

	// Mount temp dir.
	outDir, err := ioutil.TempDir("", "stream2me")
	if err != nil {
		errorExit(err)
	}
	fmt.Printf("tempDir: %s\n", outDir)

	formatStr := "media_%d.ts"

	done := make(chan bool)

	go func() {
		<-done
		fmt.Println("error => done")
		os.Exit(1)
	}()

	fmt.Println("downloading...")
	from := time.Now()

	// Download chunk sequence.
	n, err := downloadStreamOptimized(baseURL, outDir, formatStr, done)
	if err != nil {
		errorExit(err)
	}

	wg.Wait()
	fmt.Printf("\nreceived %d chunks in %.1f s\n", n, time.Since(from).Seconds())

	// Merge everything together.
	err = writeConcatenatedFile(outDir, outFileName, formatStr, n)
	if err != nil {
		errorExit(err)
	}

	// Wipe temp dir.
	err = os.RemoveAll(outDir)
	if err != nil {
		errorExit(err)
	}

	fmt.Println("done!")
	os.Exit(0)
}
