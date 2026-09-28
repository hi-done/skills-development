package main

import (
	_ "dc_backend/internal/logic"

	"github.com/gogf/gf/v2/os/gctx"

	"dc_backend/internal/cmd"

	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
