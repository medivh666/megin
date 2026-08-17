package router

import (
	"megin/internal/api"
	"megin/internal/middleware"
	articleModule "megin/internal/module/article"
	"megin/pkg/context/router"
)

// 前端API
func InitApiRouter(routeRegistry *router.RouteRegistry) *router.RouteRegistry {

	//验证
	apiGroup := routeRegistry.Group("api")
	apiGroup.Use(middleware.ApiAuthTokenRequired())

	//不用验证
	noAuthGroup := routeRegistry.Group("api")

	user := &api.User{}
	router.POST(noAuthGroup, "/user/register", user.Register)
	router.POST(noAuthGroup, "/user/login", user.Login)
	router.GET(apiGroup, "/user/info", user.Info)

	article := &api.Article{}
	router.GET(noAuthGroup, "/article/detail", article.Detail, articleModule.DetailOptions...)
	router.POST(noAuthGroup, "/article/create", article.Create, articleModule.CreateOptions...)
	router.POST(noAuthGroup, "/article/update", article.Update, articleModule.UpdateOptions...)
	router.POST(noAuthGroup, "/article/delete", article.Delete, articleModule.DeleteOptions...)

	//假如这个不用验证
	router.GET(noAuthGroup, "/article/pageList", article.PageList)

	return routeRegistry
}
