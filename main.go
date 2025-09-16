package main

import (
	"places_api/cmd"
)

//	@title			Places API
//	@version		1.0
//	@description	A cache-first Places API for location search and discovery
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io

//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT

//	@host		localhost:8080
//	@BasePath	/

//	@schemes	http https

func main() {
	cmd.Execute()
}
