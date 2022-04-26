package routers

import (
	beego "github.com/beego/beego/v2/server/web"
	"github.com/robberphex/example-beego-opensergo/controllers"
)

func init() {
	beego.Router("/", &controllers.MainController{}, "get,post:Get")
	beego.Router("/a", &controllers.MainController{})
	beego.Router("/a/bc", &controllers.MainController{})
	beego.Router("/api/:id([0-9]+)", &controllers.MainController{})
}
