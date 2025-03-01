package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gitee.com/qiulaidongfeng/chatroom/go/chatroom/chatroom"
	"gitee.com/qiulaidongfeng/nonamevote/nonamevote"
	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/mux"
	"github.com/qiulaidongfeng/restart"
)

func main() {
	restart.Run(Main)
}

func Main() {
	m := mux.New()
	m.AddStd("qiulaidongfeng.ip-ddns.com", nonamevote.S.Handler())
	m.AddStd("chat.qiulaidongfeng.ip-ddns.com", chatroom.S.Handler())
	s := gin.New()
	s.NoRoute(func(ctx *gin.Context) {
		m.ServeHTTP(ctx.Writer, ctx.Request)
	})

	srv := &http.Server{
		Addr:    ":443",
		Handler: s.Handler(),
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
	err := srv.ListenAndServeTLS("./cert.pem", "./key.pem")
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
	<-end
}
