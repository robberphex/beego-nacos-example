package routers

import (
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	beecontext "github.com/beego/beego/v2/server/web/context"
	"github.com/robberphex/example-beego-opensergo/controllers"
	"net/http"
	"strings"
)

func init() {
	//beego.Router("/", &controllers.MainController{}, "get,post:Get")
	beego.Router("/", &controllers.MainController{}, "get,post:Get")
	beego.Router("/a", &controllers.MainController{})
	beego.Router("/a/bc", &controllers.MainController{})
	beego.Router("/api/:id([0-9]+)", &controllers.MainController{})

	x := beego.BeeApp.Handlers
	for i, info := range x.GetAllControllerInfo() {
		fmt.Printf("%d\t=\t%s\t%#v\n", i, info.GetPattern(), info.GetMethod())
	}

	bctx := beecontext.NewContext()
	bctx.Input.Context = beecontext.NewContext()
	req, err := http.NewRequest("GET", "/", strings.NewReader("x"))
	_ = err
	bctx.Input.Context.Request = req
	r, found := x.FindRouter(bctx)
	p := r.GetPattern()
	fmt.Println("pattern:\t" + p)
	_, _ = r, found
	_ = x
}
