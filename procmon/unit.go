package procmon

import (
	"sort"
)

type Unit struct {
	Factor int
	Suffix string
}

func NewUnit(factor int, suffix string) Unit {
	return Unit{Factor: factor, Suffix: suffix}
}

func (u Unit) ConvertValue(v interface{}) (
	result interface{},
	suffix string,
	oversized bool,
) {
	switch v.(type) {
	case string:
		result = v
		return
	case int:
		v, _ := v.(int)
		value := v / u.Factor
		oversized = int(float64(value) * 100.0) < 90
		result = value
	case int64:
		v, _ := v.(int64)
		value := v / int64(u.Factor)
		oversized = int(float64(value) * 100.0) < 90
		result = value
	case uint:
		v, _ := v.(uint)
		value := v / uint(u.Factor)
		oversized = int(float64(value) * 100.0) < 90
		result = value
	case uint64:
		v, _ := v.(uint64)
		value := v / uint64(u.Factor)
		oversized = int(float64(value) * 100.0) < 90
		result = value
	case float32:
		v, _ := v.(float32)
		value := v / float32(u.Factor)
		oversized = int(float64(value) * 100.0) < 90
		result = value
	case float64:
		v, _ := v.(float64)
		value := v / float64(u.Factor)
		oversized = int(float64(value) * 100.0) < 90
		result = value
	}
	suffix = u.Suffix
	return
}

// UnitByFactor implements sort.Interface for []Unit based on Factor field
type UnitByFactor []Unit
func (s UnitByFactor) Len() int { return len(s) }
func (s UnitByFactor) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s UnitByFactor) Less(i, j int) bool { return s[i].Factor < s[j].Factor }

func (s UnitByFactor) GetResult(v interface{}) (result interface{}, suffix string) {
	switch v.(type) {
	case string:
		result = v
		return
	}
	if len(s) == 0 {
		result = v
		return
	} else {
		// sort by Factor
		sort.Sort(s)
		Loop:
			for i, u := range s {
				// check when oversized
				value, unit, oversized := u.ConvertValue(v)
				if i == 0 {
					result = value
					suffix = unit
					continue
				}
				if oversized {
					break Loop
				}
				result = value
				suffix = unit
			}
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
