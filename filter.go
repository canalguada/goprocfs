// build +linux

/*
Copyright © 2022 David Guadalupe <guadalupe.david@gmail.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package goprocfs

import (
	"strings"
	"os"
)

type ProcFilter struct {
	scope string
	filter func(p *Proc, err error) bool
	message string
}

func (pf ProcFilter) Filter(p *Proc, err error) bool {
	return pf.filter(p, err)
}

func (pf ProcFilter) String() string {
	return pf.message
}

type Filterer interface {
	Filter(p *Proc, err error) bool
	String() string
}

func GetFilterer(scope string) ProcFilter {
	switch strings.ToLower(scope) {
	case "global":
		return ProcFilter{
			scope: "global",
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InUserSlice()
			},
			message: "processes inside any user slice",
		}
	case "system":
		return ProcFilter{
			scope: "system",
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InSystemSlice()
			},
			message: "processes inside system slice",
		}
	case "all":
		return ProcFilter{
			scope: "all",
			filter: func(p *Proc, err error) bool {
				return err == nil
			},
			message: "all processes",
		}
	}
	// Default is user
	return ProcFilter{
		scope: "user",
		filter: func(p *Proc, err error) bool {
			return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
		},
		message: "calling user processes",
	}
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
