package zmath

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zwords"
)

type Number interface {
	zint.Integer | ~float32 | ~float64
}

type Real interface {
	~float32 | ~float64
}

type Range[N int | int64 | float64] struct {
	Valid bool `json:",omitempty"`
	Min   N    `json:",omitempty"`
	Max   N    `json:",omitempty"`
}

type RangeF64 = Range[float64]

const MathDegreesToMeters = (111.32 * 1000)
const MathMetersToDegrees = 1 / MathDegreesToMeters

func RadToDeg(rad float64) float64 {
	return rad * 180 / math.Pi
}

func DegToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

func CalculateFraction(r float64, max int) (int, int) {
	var f, tBest, bBest, diffBest float64
	fmax := float64(max)
	diffBest = 10000.0
	for t := 0.0; t <= fmax; t++ {
		for b := 1.0; b <= fmax; b++ {
			f = t / b
			diff := math.Abs(f - r)
			if diff < diffBest {
				diffBest = diff
				tBest = t
				bBest = b
			}
			if f < r {
				break
			}
		}
		if f > r {
			break
		}
	}
	return int(tBest), int(bBest)
}

func CalculateFractionToString(r float64, max int, sep string) string {
	if r == -1.0 {
		return "-1"
	}
	top, bot := CalculateFraction(r, max)
	if top == 0 {
		return "0"
	}
	return fmt.Sprintf("%d%s%d", top, sep, bot)
}

func CalculateFractionFromString(str string, sep string) float64 {
	var top, bot float64
	if n, _ := fmt.Sscanf(str, "%f"+sep+"%f", &top, &bot); n == 2 {
		if bot == 0.0 {
			return 0
		}
		return top / bot
	}
	if n, _ := fmt.Sscanf(str, "%f", &top); n == 1 {
		return top
	}
	return 0.0
}

func Sign[N cmp.Ordered](n N) float64 {
	var zero N
	if n < zero {
		return -1
	}
	if n > zero {
		return 1
	}
	return 0
}

func RoundToAccuracy(f, accuracy float64) float64 { // this rounds to lower negative number)
	return math.Floor(f/accuracy) * accuracy //  + math.Copysign(accuracy/2, f)
}

func SinDeg(degrees float64) float64 {
	return math.Sin((degrees * math.Pi) / 180)
}

func CosDeg(degrees float64) float64 {
	return math.Cos((degrees * math.Pi) / 180)
}

func TanDeg(degrees float64) float64 {
	return math.Tan((degrees * math.Pi) / 180)
}

func AsinDeg(x float64) float64 {
	return math.Asin(x) * 180 / math.Pi
}

func AcosDeg(x float64) float64 {
	return math.Acos(x) * 180 / math.Pi
}

func AtanDeg(x float64) float64 {
	return math.Atan(x) * 180 / math.Pi
}

func UniqueRandomNumbersMapInRange(maxValue, count int) (nums map[int]bool) {
	nums = map[int]bool{}
	zint.Minimize(&count, maxValue)
	for len(nums) < count {
		nums[rand.Intn(maxValue)] = true
	}
	return
}

func RatioOfIntRandomRounded(ratio float32, count int) int {
	r := ratio * float32(count)
	i := int(r)
	if rand.Float32() <= r-float32(i) {
		i++
	}
	return i
}

// InterpolatedArrayRatioAtT returns what index in an array t (0-1) is.
// ratio is how much (0-1) of index value should be used and vs rest in next index value
func InterpolatedArrayRatioAtT(arrayLength int, t float64) (ratio float64, index int) {
	if t < 0.0 {
		return 1.0, 0
	}
	if t >= 1.0 {
		return 1.0, arrayLength - 1
	}
	f := float64(arrayLength-1) * t
	return 1 - (f - math.Floor(f)), int(f)
}

func Downsample(samples []float64, newSize int) (ns []float64) {
	slen := len(samples)
	var o float64
	var oi = -1
	ns = make([]float64, newSize, newSize)
	var count float64
	for i := 0; i < newSize; i++ {
		t := float64(i) / float64(newSize)
		ratio, index := InterpolatedArrayRatioAtT(slen, t)
		count += ratio
		o = samples[index] * ratio
		for j := oi + 1; j < index; j++ {
			o += samples[j]
			count++
		}
		ns[i] = o / count
		o = samples[index] * (1 - ratio)
		//		zlog.Info("Sample:", index, count, oi)
		count = (1 - ratio)
		oi = index + 1
	}
	return
}

func Sigmoid(n float64) float64 {
	return 1 - (1 / (1 + math.Pow(math.E, n*8-4)))
}

// Normalized returns a number scaled by 10's to be between 0 and 10.
// So 54321 becomes 5.4321 and 0.0123 becomes 1.23.
func Normalized(n float64) (norm, scale float64) {
	if n == 0 {
		return 0, 0
	}
	sign := 1.0
	if n < 0 {
		n = -n
		sign = -1
	}
	log := math.Floor(math.Log10(n))
	scale = math.Pow(10.0, log)
	norm = n / scale * sign
	return norm, scale
}

// NiceDividesOf divides s to e into minimum min parts.
// Good for dividing a graph into lines to show values on x and y-axis.
func NiceDividesOf(s, e float64, max int, niceIncs []float64) (start, inc float64) {
	inc = (e - s) / float64(max)
	if niceIncs == nil {
		niceIncs = []float64{1, 2.5, 5, 10}
	}
	norm, scale := Normalized(inc)
	nice := GetClosestNotSmaller(norm, niceIncs)
	inc = nice * scale
	start = RoundToModF64(s, inc)
	// zlog.Info("Incs:", s, e, max, "->", inc, norm, nice, start)
	return start, inc
}

// CellSizeInWidth calculates the size and start ois of a "cell" to fit in width with margins and spacing.
// It insets them a bit to account for integer widths/spacing not being able to fit in the exact space asked for.
func CellSizeInWidth(width, spacing, marginMin, marginMax, count int) (size, start int) {
	// ow := width
	width -= marginMin
	width -= marginMax
	s := (width - spacing*(count-1)) / count
	w := s*count + spacing*(count-1)
	x := (width - w) / 2
	// zlog.Warn("CellSizeInWidth:", s, "width:", width, "ow:", ow, "count:", count, "w", w, "x:", x, "spacing:", spacing, "margs", marginMin, marginMax)
	return s, x + marginMin
}

// CellSizeInWidthF is a convenience function to call CellSizeInWidth with float64's
func CellSizeInWidthF(width, spacing, marginMin, marginMax float64, count int) (size, start float64) {
	isize, istart := CellSizeInWidth(int(width), int(spacing), int(marginMin), int(marginMax), count)
	return float64(isize), float64(istart)
}

// EaseInOut makes t "ease in" gradually at 0, and easy out, flattening off near 1.
// Good for animations
func EaseInOut(t float64) float64 {
	return 1 - math.Cos(t*math.Pi)
}

// for length items, IndexOfMostFrequent compares them with each other and finds which one is used the most.
// It returns the index to an aribitrary item which is used the most.
func IndexOfMostFrequent(length int, compare func(i, j int) bool) int {
	var cmax, imax int
	var counts = make([]int, length, length)
	for i := 0; i < length; i++ {
		for j := i + 1; j < length; j++ {
			if compare(i, j) {
				counts[i]++
				if counts[i] > cmax {
					cmax = counts[i]
					cmax = i
				}
			}
		}
	}
	return imax
}

// LengthIntoDividePoints finds the point that splits len in 2, then 2 points that split in 3, 4, etc, skipping ones already used.
// Final pass adds all not used yet.
// This can be used to do a binary search. First in slice is middle, then either side of it etc.
func LengthIntoDividePoints(len int) (points []int) {
	start := time.Now()
	ln := len - 1
	l2 := len / 2
	set := make([]bool, len, len)
	points = make([]int, len, len)
	pi := 0
	for parts := 2; parts < l2; parts *= parts {
		for i := 1; i < parts; i++ {
			x := ln * i / parts
			if !set[x] {
				points[pi] = x
				pi++
				set[x] = true
			}
		}
	}
	for x := 0; x < len; x++ {
		if !set[x] {
			points[pi] = x
			pi++
			// set[x] = true -- we don't need to set it
		}
	}
	zlog.Info("Points:", len, time.Since(start))

	return
}

func RoundToMod(n, mod int) int {
	return n - n%mod
}

func RoundToMod64(n, mod int64) int64 {
	return n - n%mod
}

func RoundToModF64(n, mod float64) float64 {
	return n - math.Mod(n, mod)
}

func RoundUpToModF64(n, mod float64) float64 {
	r := math.Mod(n, mod)
	if r == 0 {
		return n
	}
	return n + (mod - r)
}

// GetNextOfSliceCombinations takes n slices of values, and the current value of each one.
// It increments to the next value of first set, or 0th and next of second set etc.
// Returning all possible combinations of the slices, wrapping to start over again.
// The slices need to always be given with the same order.
func GetNextOfSliceCombinations[S comparable](sets [][]S, current ...*S) {
	indexes := make([]int, len(sets))
	for i, eachSet := range sets {
		for j, s := range eachSet {
			if s == *current[i] {
				indexes[i] = j
			}
		}
	}
	// zlog.Info("indexes:", indexes)
	for i := range indexes {
		top := i == len(sets)-1
		inc := indexes[i] < len(sets[i])-1
		if inc || top {
			indexes[i]++
			if top && !inc {
				// zlog.Info("top!", i, len(sets))
				indexes[i] = 0
			}
			for j := 0; j < i; j++ {
				indexes[j] = 0
			}
			break
		}
	}
	for i, index := range indexes {
		*current[i] = sets[i][index]
	}
}

func GetClosestNotSmaller(n float64, to []float64) float64 {
	best := zfloat.Undefined
	for _, t := range to {
		a := t - n
		if a >= 0 && (best == zfloat.Undefined || a < best) {
			best = t
		}
	}
	return best
}

func GetClosestTo(n float64, to []float64) float64 {
	best := -1
	for i, t := range to {
		a := math.Abs(n - t)
		if best == -1.0 || a < math.Abs(n-to[best]) {
			best = i
		}
	}
	return to[best]
}

func Swap[N any](a, b *N) {
	t := *a
	*a = *b
	*b = t
}

func MakeRange[N int | int64 | float64](min, max N) Range[N] {
	return Range[N]{Valid: true, Min: min, Max: max}
}

func (r Range[N]) Length() N {
	return r.Max - r.Min
}

func (r Range[N]) Added(n N) Range[N] {
	if !r.Valid {
		r.Valid = true
		r.Min = n
		r.Max = n
		return r
	}
	r.Min = min(r.Min, n)
	r.Max = max(r.Max, n)
	return r
}

func (r *Range[N]) Add(n N) {
	*r = r.Added(n)
}

func (r *Range[N]) NiceString(digits int) string {
	if !r.Valid {
		return "invalid"
	}
	min := zwords.NiceFloat(float64(r.Min), digits)
	max := zwords.NiceFloat(float64(r.Max), digits)
	return min + " - " + max
}

// Contracted reduces the range to only just include n at min or max, depending on which is nearer.
// An n outside does nothing.
func (r Range[N]) Contracted(n N) Range[N] {
	if !r.Valid {
		return r
	}
	dMax := r.Max - n
	dMin := n - r.Min
	if dMax > 0 && dMax < dMin {
		r.Max = n
		return r
	}
	if dMin > 0 {
		r.Min = n
		return r
	}
	return r
}

func (r *Range[N]) Contract(n N) {
	*r = r.Contracted(n)
}

// ContractedExcludingInt is like Contracted, but reduces to, but doesn't include nearest int64
func (r Range[N]) ContractedExcludingInt(n int64) Range[N] {
	if !r.Valid {
		return r
	}
	dMax := int64(r.Max) - n
	dMin := n - int64(r.Min)
	if dMax > 0 && dMax < dMin {
		r.Max = N(n) - 1
		return r
	}
	if dMin > 0 {
		r.Min = N(n) + 1
		return r
	}
	return r
}

func (r *Range[N]) ContractExcludingInt(n int64) {
	*r = r.ContractedExcludingInt(n)
}

func GetRangeMins[N int | int64 | float64](rs []Range[N]) []N {
	var all []N
	for _, r := range rs {
		if r.Valid {
			all = append(all, r.Min)
		}
	}
	return all
}

func GetRangeMaxes[N int | int64 | float64](rs []Range[N]) []N {
	var all []N
	for _, r := range rs {
		if r.Valid {
			all = append(all, r.Max)
		}
	}
	return all
}

type numeric interface {
	Number | time.Duration
}

func Abs[T numeric](x T) T {
	return T(math.Float64frombits(math.Float64bits(float64(x)) &^ (1 << 63)))
}

func AbsMin[T numeric](a, b T) T {
	if Abs(a) < Abs(b) {
		return a
	}
	return b
}

func NiceNumberString(a any) (string, bool) {
	if zint.IsInt(a) {
		i := reflect.ValueOf(a).Int()
		return zint.MakeHumanFriendly(i), true
	}
	if zfloat.IsFloat(a) {
		return fmt.Sprintf("%f", a), true
	}
	return "", false
}

var countMap = map[string]map[string]int{}

// For a given group and key GetDistinctCountForKeyGroup returns a increasing number for each key asked for within each group
func GetDistinctCountForKeyGroup(group, key any) int {
	sgroup := fmt.Sprint(group)
	skey := fmt.Sprint(key)
	m := countMap[sgroup]
	if key == nil {
		return len(m)
	}
	if m == nil {
		m = map[string]int{}
	}
	n, got := m[skey]
	if got {
		return n
	}
	n = len(m) + 1
	// zlog.Info("GetDistinctCountForKeyGroup:", key, group, n, m, countMap)
	m[skey] = n
	countMap[sgroup] = m
	return n
}
