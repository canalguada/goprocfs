package procmon

import (
	"fmt"
	"strings"
	"os"
	"sync"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

func GetWaitGroup() (wg sync.WaitGroup) {
	return
}

// DbusStatus
type DbusStatus struct {
	Label, Text string
}

func GetDbusStatus(s *Status) DbusStatus {
	return DbusStatus{Label: s.Label, Text: s.Text()}
}
// DbusStatus end

type StatusUpdater func(chanStatus chan Status, wg sync.WaitGroup)

// DbusObject
type DbusObject struct {
	Resources map[string]*Resource
	Routines map[string]StatusUpdater	`dbus:"-"`
	Channel chan Status								`dbus:"-"`
	names map[string]string						`dbus:"-"`
	ordered []string									`dbus:"-"`
	timebased []*Resource							`dbus:"-"`
}

func NewObject(statuses chan Status) DbusObject {
	o := DbusObject{}
	o.Resources = make(map[string]*Resource)
	o.Routines = make(map[string]StatusUpdater)
	o.Channel = statuses
	o.names = make(map[string]string)
	return o
}

func (o *DbusObject) AddTagName(tag, name string) {
	o.names[tag] = name
	o.ordered = append(o.ordered, tag)
}

func (o *DbusObject) AddFileResource(paths ...string) {
	for _, path := range paths { // file kind
		rc := NewFileResource(path)
		for _, tag := range rc.ordered {
			o.AddTagName(tag, rc.Name)
		}
		o.Resources[rc.Name] = rc
		o.timebased = append(o.timebased, rc)
	}
}

func (o *DbusObject) UpdateTimeBased(elapsed int) {
	for _, rc := range o.timebased {
		if elapsed % rc.Seconds == 0 {
			rc.FileUpdate()
			rc.RefreshStatuses(o.Channel)
		}
	}
}

func (o *DbusObject) AddSimpleResource(rc *Resource, updater StatusUpdater) {
	for _, tag := range rc.ordered {
		o.AddTagName(tag, rc.Name)
	}
	o.Resources[rc.Name] = rc
	if updater != nil {
		o.Routines[rc.Name] = updater
	}
}

func (o *DbusObject) Close() {
	// not implemented
}

func (o DbusObject) GetStatuses() ([]*Status, *dbus.Error) {
	var result []*Status
	for _, rc := range o.Resources {
		for _, s := range rc.Statuses {
			result = append(result, s)
		}
	}
	return result, nil
}

func (o DbusObject) GetTags() (string, *dbus.Error) {
	var result []string
	for _, tag := range o.ordered {
		result = append(
			result,
			((o.Resources[o.names[tag]]).Statuses[tag]).Tagged(),
		)
	}
	return strings.Join(result, "\n"), nil
}

func (o *DbusObject) SetStatus(status Status) {
	for _, rc := range o.Resources {
		if _, found := rc.Statuses[status.Tag]; found {
			rc.UpdateStatus(status)
			break
		}
	}
}

func (o DbusObject) GetTag(tag string) (Content, *dbus.Error) {
	var result Content
	if name, found := o.names[tag]; found {
		result = ((o.Resources[name]).Statuses[tag]).Content
	}
	return result, nil
}
// DbusObject


type Service struct {
	Object DbusObject
	Conn *dbus.Conn							`dbus:"-"`
	busname string							`dbus:"-"`
	iface string								`dbus:"-"`
	path dbus.ObjectPath				`dbus:"-"`
}

func NewService(statuses chan Status, busname, iface, path string) *Service {
	s := &Service{
		Object: NewObject(statuses),
		busname: busname, // was `com.github.canalguada.gostatuses`
		iface: iface, // was `com.github.canalguada.gostatuses`
		path: dbus.ObjectPath(path), // was `/com/github/canalguada/gostatuses`
	}
	return s
}

func (s *Service) BusName() string {
	return s.busname
}

func (s *Service) Connect() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		panic(err)
	}
	s.Conn = conn
}

// Using properties
func (s *Service) BuildPropsSpec() (propsSpec prop.Map) {
	propsSpec = make(prop.Map)
	propsSpec[s.iface] = make(map[string]*prop.Prop)
	for _, rc := range s.Object.Resources {
		for _, status := range rc.Statuses {
			propsSpec[s.iface][status.Tag] = &prop.Prop{
				GetDbusStatus(status),
				false,
				prop.EmitTrue,
				nil,
			}
		}
	}
	return
}

// Using methods
// func (o Object) CpuPercent() (string, *dbus.Error) {
//   return o.GetTag(`CpuPercent`)
// }
//
// func (o Object) UpTotal() (string, *dbus.Error) {
//   return o.GetTag(`UpTotal`)
// }
//
// func (s *Service) getIntroSpec() (intro string) {
//   var methods = []string{
//     `<method name="GetStatuses">
//       <arg direction="out" type="a{sa{s((ss)s)}}"/>
//     </method>`,
//     `<method name="GetTags">
//       <arg direction="out" type="s"/>
//     </method>`,
//     `<method name="GetTag">
//       <arg direction="in" type="(ss)"/>
//       <arg direction="out" type="s"/>
//     </method>`,
//   }
//   for tag, _ := range s.Object.names {
//     methods = append(methods, fmt.Sprintf(
//       `<method name="%s">
//       <arg direction="out" type="s"/>
//     </method>`,
//       tag,
//     ))
//   }
//   intro = `<node>
//   <interface name="` + s.iface + `">
//     ` + strings.Join(methods, `
//     `) + `
//   </interface>` + introspect.IntrospectDataString + `</node>`
//   return
// }

func (s *Service) Run(debugFlag *bool) {
	// s.Connect()
	defer s.Conn.Close()
	err := s.Conn.Export(s.Object, s.path, s.iface)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "export object failed: %v\n", err)
		os.Exit(1)
	}

	// Using properties
	props, err := prop.Export(s.Conn, s.path, s.BuildPropsSpec())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "export properties failed: %v\n", err)
		os.Exit(1)
	}
	n := &introspect.Node{
		Name: string(s.path),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       s.iface,
				Methods:    introspect.Methods(s.Object),
				Properties: props.Introspection(s.iface),
			},
		},
	}
	err = s.Conn.Export(introspect.NewIntrospectable(n), s.path,
		"org.freedesktop.DBus.Introspectable")

	// Using methods
	// err = s.Conn.Export(introspect.Introspectable(s.getIntroSpec()), s.path,
	//   "org.freedesktop.DBus.Introspectable")


	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "export introspect failed: %v\n", err)
		os.Exit(1)
	}

	reply, err := s.Conn.RequestName(s.busname, dbus.NameFlagDoNotQueue)
	if err != nil {
		panic(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		_, _ = fmt.Fprintln(os.Stderr, "name already taken")
		os.Exit(1)
	}

	// spin up workers
	wg := GetWaitGroup()
	for name, updater := range s.Object.Routines {
		if _, found := s.Object.Resources[name]; found {
			wg.Add(1)
			fmt.Println("Launching", name, "resource updater...")
			go updater(s.Object.Channel, wg)
		}
	}
	// Listening to chan Status
	fmt.Println("Listening on", s.iface, "/", s.path, "...")
	for status := range s.Object.Channel {
		// print status
		if *debugFlag {
			fmt.Println(status.Tagged())
		}
		// update status
		// Using properties
		props.SetMust(s.iface, status.Tag, GetDbusStatus(&status))
		s.Object.SetStatus(status)
	}
	wg.Wait() // wait on the workers to finish
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
