package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gitee.com/qiulaidongfeng/chatroom/go/chatroom"
	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/mux"
	"github.com/qiulaidongfeng/nonamevote/nonamevote"
	"github.com/qiulaidongfeng/restart"
)

func main() {
	restart.Run(Main)
}

var encryptS = gin.Default()

var blogS = gin.Default()

var wxbotS = gin.Default()

var aflS *gin.Engine

var refuse = errors.New("refuse")

func Main() {
	encryptS.StaticFS("", gin.Dir("wasm", false))
	blogS.StaticFS("", gin.Dir("blog", false))

	m := mux.New()
	m.AddStd("qiulaidongfeng.ip-ddns.com", nonamevote.S.Handler())
	m.AddStd("chat.qiulaidongfeng.ip-ddns.com", chatroom.S.Handler())
	m.AddStd("encrypt.qiulaidongfeng.ip-ddns.com", encryptS.Handler())
	m.AddStd("blog.qiulaidongfeng.ip-ddns.com", blogS.Handler())
	m.AddStd("wxbot.qiulaidongfeng.ip-ddns.com", wxbotS.Handler())
	m.AddStd("afl.qiulaidongfeng.ip-ddns.com", aflS.Handler())

	cert, err := tls.LoadX509KeyPair("./cert.pem", "./key.pem")
	if err != nil {
		log.Fatalf("Failed to load certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// 检查 SNI 是否被允许访问，如果不是拒绝连接
			if !m.Allow(info.ServerName) {
				return nil, refuse
			}

			// 返回证书
			return &cert, nil
		},
		MinVersion: tls.VersionTLS12, // 设置最低 TLS 版本
	}

	srv := &http.Server{
		Addr:      ":443",
		TLSConfig: tlsConfig,
		Handler:   m,
	}
	end := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGBUS)
		<-c
		fmt.Println("正在关机")
		err := srv.Shutdown(context.Background())
		if err != nil {
			slog.Error("", "err", err)
		}
		nonamevote.Close()
		close(end)
		fmt.Println("关机完成")
	}()
	err = srv.ListenAndServeTLS("", "")
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
	<-end
}
