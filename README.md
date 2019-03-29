# stream2me

Package **stream2me** is a simple [MPEG-2](https://en.wikipedia.org/wiki/MPEG_transport_stream) command line tool that downloads and concatenates .ts stream chunks.

## Status

V1.0

## Installation

Required build version: go1.9 and up

`go get github.com/hhrutter/stream2me/...`

## Usage

```
Usage: stream2me outFile baseUrl

outFile ... the output .ts file

baseUrl ... the url for fetching .ts chunks
````

Example: `stream2me test.ts https://xyz.org/content/test_` 

<p>
  <img src="resources/screenshot.png" width="500">
</p>

**stream2me** will go and fetch chunks using a format string that will be appended to the url. There has to be a counter as part of the format string. In the example above using the default format string `%d.ts` **stream2me** will fetch the urls: 

```
https://xyz.org/content/test_0.ts
https://xyz.org/content/test_1.ts
https://xyz.org/content/test_2.ts
```
 and so forth but not necessarily in that order.

Once all chunks are downloaded **stream2me** will concatenate them and write out the result to `test.ts` in this example.

You may have to patch the format string to your needs.

## License

MIT

## Powered By

<p align="center">
  <a href="https://golang.org"> <img src="resources/Go-Logo_Aqua.png" width="200"> </a>
</p>
