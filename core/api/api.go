package api

import (
	"crypto/subtle"
	"net/http"
	"sync"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/gin-gonic/gin"
)

type Server struct{}

func (s *Server) Start(wg *sync.WaitGroup) {
	r := gin.Default()

	// Basicauth in 2022 hahahahah
	// Yes I lol with you
	// but w/e
	{
		authorized := r.Group("/", currentBasicAuth())

		authorized.StaticFS("message", http.Dir("files/recording/message"))
		authorized.StaticFS("random", http.Dir("files/recording/random"))
	}

	r.Run()
}

func currentBasicAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		username, password, ok := ctx.Request.BasicAuth()
		expected := secrets.Current().HTTPServerAuth
		valid := ok && expected.Username != "" && expected.Password != "" &&
			subtle.ConstantTimeCompare([]byte(username), []byte(expected.Username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(expected.Password)) == 1
		if !valid {
			ctx.Header("WWW-Authenticate", `Basic realm="benis-phone"`)
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Next()
	}
}
