package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gosuri/uilive"
)

const (
	top    = "\u2581"
	bottom = "\u2594"
	left   = "\u258F"
	right  = "\u2595"
	box    = "\u2588"
	b1     = "\u258F"
	b2     = "\u258E"
	b3     = "\u258D"
	b4     = "\u258C"
	b5     = "\u258B"
	b6     = "\u258A"
	b7     = "\u2589"
	b8     = "\u2588"

	formatStr        = "%d.ts" // Chunk file name.
	progressBarWidth = 40      // Progress bar width in characters.
)

var (
	wg  sync.WaitGroup
	mtx sync.Mutex
	st  stats
)

type intSet map[int]bool

type stats struct {
	set   intSet
	max   int
	count int
	from  time.Time
}

func (st *stats) percentage() float64 {
	return float64(len(st.set)) / float64(st.max) * 100
}

func (st *stats) add(i int) {
	st.set[i] = true
	if i > st.max {
		st.max = i
	}
	st.count++
}

func (st *stats) setForRange(from, thru int) bool {
	for i := from; i < thru; i++ {
		if !st.set[i] {
			return false
		}
	}
	return true
}

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

	// TODO Bail out if outfile already exists.

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
	url = baseURL + fileName
	filePath = filepath.Join(outDir, fileName)
	return
}

func drawTopRow(pw io.Writer, w int) {
	for i := 0; i < w; i++ {
		fmt.Fprint(pw, top)
	}
	fmt.Fprintln(pw)
}

func drawBottomRow(pw io.Writer, w int) {
	for i := 0; i < w; i++ {
		fmt.Fprint(pw, bottom)
	}
	fmt.Fprintln(pw)
}

func drawFineProgressBar(pw io.Writer, w int, percentage float64, from time.Time) {

	drawTopRow(pw, w)

	var appendLeft bool

	c := float64(w) / 100 * percentage

	s := fmt.Sprintf(" %.0f%% ", percentage)
	j := w/2 - len(s)/2

	for i := 0; i < w; i++ {
		if i >= j && i < j+len(s) {
			fmt.Fprint(pw, string(s[i-j]))
			continue
		}
		if float64(i) < c {
			d := c - float64(i)
			if d >= 1 {
				fmt.Fprint(pw, box)
				continue
			}
			switch int(d / .125) {
			case 0:
				fmt.Fprint(pw, " ")
			case 1:
				fmt.Fprint(pw, b1)
			case 2:
				fmt.Fprint(pw, b2)
			case 3:
				fmt.Fprint(pw, b3)
			case 4:
				fmt.Fprint(pw, b4)
			case 5:
				fmt.Fprint(pw, b5)
			case 6:
				fmt.Fprint(pw, b6)
			case 7:
				fmt.Fprint(pw, b7)
			}
			if i == w-1 {
				appendLeft = true
			}
			continue
		}
		if i == 0 {
			fmt.Fprint(pw, left)
			continue
		}
		if i == w-1 {
			fmt.Fprint(pw, right)
			continue
		}

		fmt.Fprint(pw, " ")
	}

	if appendLeft {
		fmt.Fprint(pw, left)
	}

	d := int(time.Since(from).Seconds())
	if d >= 60 {
		m := d / 60
		s := d % 60
		fmt.Fprintf(pw, " elapsed: %dm %ds chunks: %d\n", m, s, len(st.set))
	} else {
		fmt.Fprintf(pw, " elapsed: %ds chunks: %d\n", d, len(st.set))
	}

	drawBottomRow(pw, w)
}

func showStandardProgressBar(pw io.Writer, st *stats, w int) {
	drawFineProgressBar(pw, w, st.percentage(), st.from)
}

func rangeFor(w, m, i, j int) (float64, float64) {

	from := float64(i * m / w)
	thru := float64((i + 1) * m / w)

	return from, thru
}

func showChunkedProgressBar(pw io.Writer, st *stats, w int) {

	s := fmt.Sprintf(" %.0f%% ", st.percentage())
	j := w/2 - len(s)/2

	drawTopRow(pw, w)

	for i := 0; i < w; i++ {
		if i >= j && i < j+len(s) {
			fmt.Fprint(pw, string(s[i-j]))
			continue
		}
		from, thru := rangeFor(w, st.max, i, i+1)
		if st.setForRange(int(from)+1, int(thru)) {
			fmt.Fprint(pw, box)
			continue
		}
		if i == 0 {
			fmt.Fprint(pw, left)
			continue
		}
		if i == w-1 {
			fmt.Fprint(pw, right)
			continue
		}
		fmt.Fprint(pw, " ")
	}
	fmt.Fprintln(pw)

	drawBottomRow(pw, w)
}

func showProgress(pw io.Writer, st *stats) {
	showStandardProgressBar(pw, st, progressBarWidth)
	//showChunkedProgressBar(pw, st, progressBarWidth)
}

func downloadChunk(baseURL, outDir, formatStr string, i int, pw *uilive.Writer) (int, error) {

	url, filePath := urlAndFilePath(baseURL, outDir, fmt.Sprintf(formatStr, i))

	n, err := download(url, filePath)
	if err != nil {
		return 0, err
	}

	if n > 0 {
		mtx.Lock()
		st.add(i)
		showProgress(pw, &st)
		pw.Flush()
		mtx.Unlock()
	}

	return n, nil
}

func downloadChunks(baseURL, outDir, formatStr string, startInd, count int, done chan bool, pw *uilive.Writer) error {

	defer wg.Done()

	for i := startInd; i < startInd+count; i++ {

		n, err := downloadChunk(baseURL, outDir, formatStr, i, pw)
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

func downloadStream(baseURL, outDir, formatStr string, pw *uilive.Writer) (chunks int, err error) {

	i := 0

	for ; ; i++ {

		n, err := downloadChunk(baseURL, outDir, formatStr, i, pw)
		if err != nil {
			return 0, err
		}

		if n == 0 {
			break
		}

	}

	return i, nil
}

func downloadStreamOptimized(baseURL, outDir, formatStr string, done chan bool, pw *uilive.Writer) (chunks int, err error) {

	i := 0
	step := 100

	for {

		n, err := downloadChunk(baseURL, outDir, formatStr, i+step-1, pw)
		if err != nil {
			return 0, err
		}

		if n > 0 {
			wg.Add(1)
			go downloadChunks(baseURL, outDir, formatStr, i, step-1, done, pw)
			i += step
			continue
		}

		if step > 2 {
			step /= 2
			continue
		}

		if step == 2 {
			n, err = downloadChunk(baseURL, outDir, formatStr, i, pw)
			if err != nil {
				return 0, err
			}
		}

		if n > 0 {
			return i + 1, nil
		}

		return i, nil
	}

}

func main() {

	// Process command line arguments.
	if len(os.Args) != 3 {
		errorExit(errors.New("Usage: stream2me outFile baseUrl"))
	}
	baseURL := os.Args[2]
	outFileName := os.Args[1]

	// Mount temp dir.
	outDir, err := ioutil.TempDir("", "stream2me")
	if err != nil {
		errorExit(err)
	}
	//fmt.Printf("tempDir: %s\n\n", outDir)

	pw := uilive.New()

	done := make(chan bool)

	go func() {
		<-done
		fmt.Println("error => done")
		os.Exit(1)
	}()

	st = stats{
		set:  intSet{},
		from: time.Now(),
	}

	// Download chunk sequence.
	n, err := downloadStreamOptimized(baseURL, outDir, formatStr, done, pw)
	if err != nil {
		errorExit(err)
	}

	wg.Wait()

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

	os.Exit(0)
}
