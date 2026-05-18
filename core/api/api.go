package api

import (
	"net/http"
	"sync"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/gin-gonic/gin"
)

type Server struct{}

func (s *Server) Start(wg *sync.WaitGroup) {
	credentials := secrets.Loaded

	r := gin.Default()

	// Basicauth in 2022 hahahahah
	// Yes I lol with you
	// but w/e
	{
		authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
			credentials.HTTPServerAuth.Username: credentials.HTTPServerAuth.Password,
		}))

		authorized.StaticFS("message", http.Dir("files/recording/message"))
		authorized.StaticFS("random", http.Dir("files/recording/random"))
	}

	r.Run()
}
