package debug

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
)

var Enabled bool

func init() {
	Enabled = true
}

func Start(addr string) {
	go http.ListenAndServe(addr, nil)
}

func RegisterHTTPHandler(pattern string, handler http.Handler) {
	if Enabled {
		http.Handle(pattern, handler)
	}
}

func genRandomID() string {
	c := 5
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func Enter_(args ...interface{}) string {
	pc, _, _, _ := runtime.Caller(1)
	functionObject := runtime.FuncForPC(pc)
	extractFnName := regexp.MustCompile(`^.*\/(.*)$`)
	fnName := extractFnName.ReplaceAllString(functionObject.Name(), "$1")
	argsStr := make([]string, len(args))
	for idx, arg := range args {
		argsStr[idx] = fmt.Sprintf("%+v", arg)
	}

	fnWithId := genRandomID() + " " + fnName

	fmt.Printf("==> %s(%s)\n", fnWithId, strings.Join(argsStr, ","))
	return fnWithId
}

func Exit_(fnName string) {
	fmt.Printf("<== %s\n", fnName)
}
