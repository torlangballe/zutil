package zgeo

type DropShadow struct {
	Delta  Size
	Blur   float64
	Color  Color
	Inset  bool
	Spread float64
}

var (
	DropShadowUndef = DropShadow{Delta: SizeUndef, Blur: -1}
	DropShadowClear = DropShadow{}
)

func MakeDropShadow(dx, dy, blur float64, col Color) DropShadow {
	return DropShadow{Delta: SizeD(dx, dy), Blur: blur, Color: col}
}
