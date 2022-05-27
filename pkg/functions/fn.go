package functions

type Fn struct {
	Name        string
	Prefix      Prefix
	Exit        string
	InputLength int

	Actions []Action
}

func (f *Fn) IndexActions() {
	for i, val := range f.Actions {
		if val.Num == 0 {
			f.Actions[i].Num = i
		}
	}
}

func (f *Fn) RunPrefix() {

}
