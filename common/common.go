package common

import log "github.com/sirupsen/logrus"

func ShouldSucc(err error) {
	if err != nil {
		log.Panic(err.Error())
	}
}
