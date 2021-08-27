package debug

import (
	"fmt"
	"io"
)

const SEALER_DEBUG_MOTD = `

	███████╗███████╗ █████╗ ██╗     ███████╗██████╗
	██╔════╝██╔════╝██╔══██╗██║     ██╔════╝██╔══██╗
	███████╗█████╗  ███████║██║     █████╗  ██████╔╝
	╚════██║██╔══╝  ██╔══██║██║     ██╔══╝  ██╔══██╗
	███████║███████╗██║  ██║███████╗███████╗██║  ██║
	╚══════╝╚══════╝╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝
		
	██████╗ ███████╗██████╗ ██╗   ██╗ ██████╗
	██╔══██╗██╔════╝██╔══██╗██║   ██║██╔════╝
	██║  ██║█████╗  ██████╔╝██║   ██║██║  ███╗
	██║  ██║██╔══╝  ██╔══██╗██║   ██║██║   ██║
	██████╔╝███████╗██████╔╝╚██████╔╝╚██████╔╝
	╚═════╝ ╚══════╝╚═════╝  ╚═════╝  ╚═════╝
`

func showMotd(w io.Writer, motd string) {
	fmt.Fprintln(w, motd)
}
