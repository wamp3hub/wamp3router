package routerShared

import "fmt"

// ASCII Art Generated by https://www.asciiart.eu/text-to-ascii-art
// Font: ASCII Shadow
// Border: Box drawings light
const logotype = `
┌─────────────────────────────────────────────────────────────────┐
│░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░│
│░░██╗░░░░██╗░█████╗░███╗░░░███╗██████╗░██████╗░██████╗░██████╗░░░│
│░░██║░░░░██║██╔══██╗████╗░████║██╔══██╗╚════██╗██╔══██╗██╔══██╗░░│
│░░██║░█╗░██║███████║██╔████╔██║██████╔╝░█████╔╝██████╔╝██║░░██║░░│
│░░██║███╗██║██╔══██║██║╚██╔╝██║██╔═══╝░░╚═══██╗██╔══██╗██║░░██║░░│
│░░╚███╔███╔╝██║░░██║██║░╚═╝░██║██║░░░░░██████╔╝██║░░██║██████╔╝░░│
│░░░╚══╝╚══╝░╚═╝░░╚═╝╚═╝░░░░░╚═╝╚═╝░░░░░╚═════╝░╚═╝░░╚═╝╚═════╝░░░│
│░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░│
└─────────────────────────────────────────────────────────────────┘
`

func PrintLogotype() {
	fmt.Print(logotype)
}
