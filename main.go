package main

import (
	"flag"
	"io/ioutil"
	"log"
	"tinywebserver/server"
)

func init() {
	//TODO: Probably add more sophiscicated logging system in the future
	//Also the performance of this type of logging should be measured
	enableLogs := flag.Bool("logging", false, "If true, will enable logs")
	flag.Parse()
	if !(*enableLogs) {
		log.SetOutput(ioutil.Discard)
		log.SetFlags(0)
	}
}

func main() {
	server.Start()
}