package color

// https://ethanschoonover.com/solarized/#the-values
var (
	Yellow  = NewTrueColor("#B58900")
	Orange  = NewTrueColor("#CB4B16")
	Red     = NewTrueColor("#DC322F")
	Magenta = NewTrueColor("#D33682")
	Violet  = NewTrueColor("#6C71C4")
	Blue    = NewTrueColor("#268BD2")
	Cyan    = NewTrueColor("#2AA198")
	Green   = NewTrueColor("#859900")

	XXXLight = NewAdaptiveTrueColor("#FDF6E3", "#002B36") // base3
	XXLight  = NewAdaptiveTrueColor("#EEE8D5", "#073642") // base2
	XLight   = NewAdaptiveTrueColor("#93A1A1", "#586E75") // base1
	Light    = NewAdaptiveTrueColor("#839496", "#657B83") // base0
	Dark     = NewAdaptiveTrueColor("#657B83", "#839496") // base00
	XDark    = NewAdaptiveTrueColor("#586E75", "#93A1A1") // base01
	XXDark   = NewAdaptiveTrueColor("#073642", "#EEE8D5") // base02
	XXXDark  = NewAdaptiveTrueColor("#002B36", "#FDF6E3") // base03
)

