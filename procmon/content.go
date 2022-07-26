package procmon

type Content struct {
	Label string
	Value interface{}
	MainUnit string
	Units []Unit
}

func (c *Content) AddUnit(u Unit) {
	if len(c.Units) == 0 {
		c.MainUnit = u.Suffix
	}
	c.Units = append(c.Units, u)
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
