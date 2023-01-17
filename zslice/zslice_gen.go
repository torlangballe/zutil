//go:build go1.18

package zslice

func Add[T any](s *[]T, a T) {
	*s = append(*s, a)
}
