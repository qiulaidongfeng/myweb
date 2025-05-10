package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unique"

	"gitee.com/qiulaidongfeng/chatroom/go/chatroom"
	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/nonamevote/nonamevote"
	"github.com/qiulaidongfeng/wxbot/wxbot"
)

func init() {
	nonamevote.S = gin.New()
	nonamevote.S.Use(gin.Recovery(), Logger("./log"))
	nonamevote.Handle(nonamevote.S)

	chatroom.S = gin.New()
	chatroom.S.Use(gin.Recovery(), Logger("./chatlog"))
	chatroom.Handle(chatroom.S)

	encryptS = gin.New()
	encryptS.Use(gin.Recovery(), Logger("./encryptlog"))

	wxbotS = gin.New()
	wxbotS.Use(gin.Recovery(), Logger("./wxbotlog"))
	wxbot.Handle(wxbotS)

	runtime.GC()
	debug.FreeOSMemory()
}

var um sync.Map

func Logger(path string) func(*gin.Context) {
	fd, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	var lock sync.Mutex
	buf := bufio.NewWriter(fd)
	change := Ticker(func() {
		lock.Lock()
		defer lock.Unlock()
		buf.Flush()
	})
	return func(c *gin.Context) {
		ClientIP := unique.Make(c.RemoteIP()).Value()
		count := nonamevote.AddIpCount(ClientIP)
		expiration := nonamevote.GetExpiration()
		maxcount := nonamevote.GetMaxCount()
		if count > maxcount {
			if count == maxcount+1 {
				lock.Lock()
				defer lock.Unlock()
				fmt.Fprintf(buf, "%s access forbidden\n", ClientIP)
			}
			c.String(403, "%d秒内这个ip(%s)访问网站超过%d次，请等%d秒后再访问网站", expiration, ClientIP, maxcount, expiration)
			c.Abort()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Stop timer
		TimeStamp := time.Now()
		Latency := TimeStamp.Sub(start)

		Method := c.Request.Method
		StatusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		ug := c.Request.Header.Get("User-Agent")
		if strings.Contains(ug, "ClouDNS") {
			ug = "ClouDNS"
		}
		if strings.Contains(ug, "UptimeRobot") {
			ug = "UptimeRobot"
		}

		if _, ok := um.LoadOrStore(ClientIP, ug); ok {
			ug = ""
		} else {
			go func() { time.Sleep(4 * time.Hour); um.Delete(ClientIP) }()
		}

		lock.Lock()
		defer lock.Unlock()
		fmt.Fprintf(buf, "%s |%d| %s | %s | %s | %s | %s |\n", start.Format("2006-01-02 15-04-05"), StatusCode, Latency, ClientIP, Method, path, ug)
		change()
	}
}

// -- COPY
// 测试用
var stop, cancel = context.WithCancel(context.Background())

func Ticker(f func()) (change func()) {
	interval := atomic.Value{}
	interval.Store(10 * time.Second)
	sig := make(chan struct{})
	send := atomic.Bool{}

	go func() {
		for {
			currentInterval := interval.Load().(time.Duration)
			t := time.NewTimer(currentInterval)
			select {
			case <-sig:
				interval.Store(10 * time.Second)
			case <-t.C:
				send.Store(true)
				f()
				interval.Store(24 * time.Hour * 365)
			case <-stop.Done():
				return
			}
		}
	}()

	return func() {
		if send.CompareAndSwap(true, false) {
			sig <- struct{}{}
		}
	}
}
