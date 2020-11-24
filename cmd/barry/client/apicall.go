package client

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/OnitiFR/barry/common"
	"github.com/blang/semver"
	"github.com/c2h5oh/datasize"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar"
)

// API describes the basic elements to call the API
type API struct {
	ServerURL string
	APIKey    string
	Trace     bool
	Time      bool
}

// APICall describes a call to the API
type APICall struct {
	api           *API
	Method        string
	Path          string
	Args          map[string]string
	JSONCallback  func(io.Reader, http.Header)
	DestFilePath  string
	DestStream    *os.File
	PrintLogTopic bool
}

// NewAPI create a new API instance
func NewAPI(server string, apiKey string, trace bool, time bool) *API {
	return &API{
		ServerURL: server,
		APIKey:    apiKey,
		Trace:     trace,
		Time:      time,
	}
}

// NewCall create a new APICall
func (api *API) NewCall(method string, path string, args map[string]string) *APICall {
	return &APICall{
		api:    api,
		Method: method,
		Path:   path,
		Args:   args,
	}
}

func cleanURL(urlIn string) (string, error) {
	urlObj, err := url.Parse(urlIn)
	if err != nil {
		return urlIn, err
	}
	urlObj.Path = path.Clean(urlObj.Path)
	return urlObj.String(), nil
}

// Do the actual API call
func (call *APICall) Do() {
	method := strings.ToUpper(call.Method)

	apiURL, err := cleanURL(call.api.ServerURL + "/" + call.Path)
	if err != nil {
		log.Fatal(err)
	}

	data := url.Values{}
	for key, val := range call.Args {
		data.Add(key, val)
	}
	if call.api.Trace == true {
		data.Add("trace", "true")
	}

	var req *http.Request

	switch method {
	case "GET", "DELETE":
		finalURL := apiURL + "?" + data.Encode()
		req, err = http.NewRequest(method, finalURL, nil)
		if err != nil {
			log.Fatal(removeAPIKeyFromString(err.Error(), call.api.APIKey))
		}
	case "POST", "PUT":
		// simple URL encoded form
		req, err = http.NewRequest(method, apiURL, bytes.NewBufferString(data.Encode()))
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	default:
		log.Fatalf("apicall does not support '%s' yet", method)
	}

	req.Header.Set("Barry-Key", call.api.APIKey)
	req.Header.Set("Barry-Version", Version)
	req.Header.Set("Barry-Protocol", strconv.Itoa(ProtocolVersion))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(removeAPIKeyFromString(err.Error(), call.api.APIKey))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatalf("\nError: %s (%v)\nMessage: %s",
			resp.Status,
			resp.StatusCode,
			string(body),
		)
	}

	mime := resp.Header.Get("Content-Type")

	switch mime {
	case "text/plain", "text/plain; charset=utf-8":
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(string(body))
	case "application/json":
		if call.JSONCallback == nil {
			log.Fatalf("no JSON callback defined for %s %s", call.Method, call.Path)
		}
		call.JSONCallback(resp.Body, resp.Header)
		// return? call.callback?
	case "application/octet-stream":
		if call.DestFilePath == "" && call.DestStream == nil {
			log.Fatalf("no DestFilePath/DestStream defined for %s %s", call.Method, call.Path)
		}

		if call.DestFilePath != "" {
			err := downloadFile(call.DestFilePath, resp)
			if err != nil {
				log.Fatal(err)
			}
		} else if call.DestStream != nil {
			_, err := io.Copy(call.DestStream, resp.Body)
			if err != nil {
				log.Fatal(err)
			}
		}
	default:
		log.Fatalf("unsupported content type '%s'", mime)
	}

	latestClientVersionKnownByServer := resp.Header.Get("Latest-Known-Client-Version")
	if latestClientVersionKnownByServer != "" {
		verFromServer, err1 := semver.Make(latestClientVersionKnownByServer)
		verSelf, err2 := semver.Make(Version)
		if err1 == nil && err2 == nil && verFromServer.GT(verSelf) {
			green := color.New(color.FgHiGreen).SprintFunc()
			yellow := color.New(color.FgHiYellow).SprintFunc()
			msg := fmt.Sprintf("According to the server, a client update is available: %s → %s\n", yellow(Version), green(latestClientVersionKnownByServer))
			msg = msg + fmt.Sprintf("Update:\n    go get -u github.com/OnitiFR/barry/cmd/barry\n")
			GetExitMessage().Message = msg
		}
	}
}

// TimestampShow allow to override -d / --time flag
func (call *APICall) TimestampShow(show bool) {
	call.api.Time = show
}

func removeAPIKeyFromString(in string, key string) string {
	if key == "" {
		return in
	}
	return strings.Replace(in, key, "xxx", -1)
}

func downloadFile(filename string, resp *http.Response) error {
	if common.PathExist(filename) == true {
		return fmt.Errorf("error: file '%s' already exists", filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	fmt.Printf("downloading %s…\n", filename)

	var bar io.Writer
	if isatty.IsTerminal(os.Stdout.Fd()) {
		bar = progressbar.DefaultBytes(
			resp.ContentLength,
			"",
		)
	} else {
		bar = ioutil.Discard
	}

	bytesWritten, err := io.Copy(io.MultiWriter(file, bar), resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("finished, downloaded %s\n", (datasize.ByteSize(bytesWritten) * datasize.B).HR())
	return nil
}
