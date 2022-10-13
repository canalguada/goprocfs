package procmon

import (
	"fmt"
)

type Status struct {
	Content
	Tag    string
	format string
}

func NewStatus(tag, label string) *Status {
	return &Status{Content: Content{Label: label}, Tag: tag, format: "%v"}
}

func (s *Status) SetFormat(format string) {
	s.format = format
}

func (s *Status) SetLabel(label string) {
	s.Label = label
}

func (s *Status) SetValue(v interface{}) {
	s.Value = v
}

func (s *Status) Text() (txt string) {
	if s.Value == nil {
		return
	}
	var (
		result interface{}
		suffix string
	)
	if len(s.Units) > 0 {
		result, suffix = UnitByFactor(s.Units).GetResult(s.Value)
	} else {
		result = s.Value
	}
	txt = fmt.Sprintf(s.format, result)
	if len(suffix) > 0 {
		txt += suffix
	}
	return
}

func (s *Status) String() (txt string) {
	if len(s.Label) > 0 {
		txt = s.Label + " "
	}
	txt += s.Text()
	return
}

func (s *Status) Tagged() (txt string) {
	return s.Tag + ": " + s.String()
}

func (s *Status) Property() string {
	return fmt.Sprintf("%s:%s", s.Label, s.Text())
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
