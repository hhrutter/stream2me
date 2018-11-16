package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

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

// download url to fileName.
func download(url, fileName string) (int, error) {

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
		errorExit(fmt.Errorf("we need to abort => received http error %d", resp.StatusCode))
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

	url, filePath := urlAndFilePath(baseURL, outDir, fmt.Sprintf(formatStr, i))

	n, err := download(url, filePath)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func downloadChunks(baseURL, outDir, formatStr string, from, thru int) error {

	for i := from; i <= thru; i++ {

		url, filePath := urlAndFilePath(baseURL, outDir, fmt.Sprintf(formatStr, i))

		n, err := download(url, filePath)
		if err != nil {
			return err
		}

		if n == 0 {
			errorExit(fmt.Errorf("Unknown chunk %d", i))
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
		if i%100 == 0 {
			fmt.Printf(" %d\n", i)
		} else if i%10 == 0 {
			fmt.Print("\n")
		}

		if n == 0 {
			break
		}

	}

	return i, nil
}

func downloadStreamAlt(baseURL, outDir, formatStr string) (chunks int, err error) {

	i := 0
	step := 10

	for {
		n, err := downloadChunk(baseURL, outDir, formatStr, i+step)
		if err != nil {
			return 0, err
		}

		if n > 0 {
			from := 0
			if i > 0 {
				from = i + 1
			}
			go downloadChunks(baseURL, outDir, formatStr, from, i+step-1)
			i += step
			continue
		}

		if step == 1 {
			_, err := downloadChunk(baseURL, outDir, formatStr, 0)
			if err != nil {
				return 0, err
			}
			return i + step, nil
		}

		step /= 2

	}

}

func main() {

	// original:
	// https://varorfvod.sf.apa.at/cms-austria_nas/_definst_/nas/cms-austria/online/2018-11-15_1415_sd_06_Expeditionen--G_____13995356__o__7804828615__s14396742_2__ORF3HD_14203621P_15034107P_Q6A.mp4/media_3.ts?lbs=20181115221549137&ip=84.113.199.33&ua=Mozilla%252f5.0%2b(Macintosh%253b%2bIntel%2bMac%2bOS%2bX%2b10_14_1)%2bAppleWebKit%252f537.36%2b(KHTML%252c%2blike%2bGecko)%2bChrome%252f70.0.3538.77%2bSafari%252f537.36

	// ==> baseURL := "https://varorfvod.sf.apa.at/cms-austria_nas/_definst_/nas/cms-austria/online/2018-11-15_1415_sd_06_Expeditionen--G_____13995356__o__7804828615__s14396742_2__ORF3HD_14203621P_15034107P_Q6A.mp4"

	if len(os.Args) != 3 {
		errorExit(errors.New("Usage: stream2me outFile baseUrl"))
	}

	baseURL := os.Args[2]
	fmt.Printf("baseURL: %s\n", baseURL)

	outFileName := os.Args[1]
	fmt.Printf("outFile: %s\n", outFileName)

	outDir, err := ioutil.TempDir("", "stream2me")
	if err != nil {
		errorExit(err)
	}
	fmt.Printf("tempDir: %s\n", outDir)

	formatStr := "media_%d.ts"

	fmt.Println("downloading...")
	from := time.Now()

	n, err := downloadStream(baseURL, outDir, formatStr)
	if err != nil {
		errorExit(err)
	}

	fmt.Printf("received %d chunks in %.1f s\n", n, time.Since(from).Seconds())

	fmt.Printf("writing %s...\n", outFileName)
	for i := 0; i < n; i++ {
		chunkFileName := filepath.Join(outDir, fmt.Sprintf(formatStr, i))
		err := appendFile(chunkFileName, outFileName)
		if err != nil {
			errorExit(err)
		}
	}

	err = os.RemoveAll(outDir)
	if err != nil {
		errorExit(err)
	}

	fmt.Println("done!")

	os.Exit(0)
}
