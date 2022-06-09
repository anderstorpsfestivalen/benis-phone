package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"gitlab.com/anderstorpsfestivalen/benis-phone/core/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/core/secrets"
)

type Server struct {
	state *controller.Controller
}

func (s *Server) Start(wg *sync.WaitGroup, ctrl *controller.Controller) {
	credentials := secrets.Loaded
	s.state = ctrl

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
