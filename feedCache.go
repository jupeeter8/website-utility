package main

import (
	"log"
	"sync"
	"time"

	"zgo.at/zcache/v2"
)

var (
	c *zcache.Cache[int, string]
	once sync.Once
)
func initcache() {
	once.Do(func() {
		c = zcache.New[int, string](30*time.Minute, 40*time.Minute)
	})

}

func cache() *zcache.Cache[int, string] {
	initcache()
	return c
}


func storeInCache(idx int, md string, latest bool) {

	c:= cache()

	if latest {
		log.Printf("id: %d is latest page entry, storing it with no expiration", idx)
		c.SetWithExpire(idx, md, zcache.NoExpiration)
		c.Touch(idx - 1)
		return
	}

	c.Set(idx, md)
}

func getFromCache(idx int) (string, bool){

	return cache().Get(idx)
}

func DeleteFromCache(idx int) {
	cache().Delete(idx)
}