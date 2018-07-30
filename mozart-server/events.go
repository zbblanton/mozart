package main

import (
  "log"
)

func eventInfo(msg ...interface{}) {
  log.Println("INFO:", msg)
}

func eventWarning(msg ...interface{}) {
  log.Println("WARNING:", msg)
}

func eventError(msg ...interface{}) {
  log.Println("ERROR:", msg)
}

func eventFatal(msg ...interface{}) {
  log.Fatalln("FATAL:", msg)
}
