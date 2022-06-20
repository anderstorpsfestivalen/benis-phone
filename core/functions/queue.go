package functions

import wr "github.com/mroth/weightedrand"

type Queue struct {
	EntryMessage    string `toml:"entrymsg"`
	Min             int
	Max             int
	Messages        []QueueMessage `toml:"msg"`
	BackgroundMusic File           `toml:"bgmusic"`
	End             Action

	rm *wr.Chooser
}

type QueueMessage struct {
	Text   string
	Weight int
}

func (q *Queue) Load() error {

	var ch []wr.Choice

	for _, c := range q.Messages {
		ch = append(ch, wr.NewChoice(c.Text, uint(c.Weight)))
	}

	chooser, err := wr.NewChooser(ch...)

	if err != nil {
		return err
	}

	q.rm = chooser

	return nil
}

func (q *Queue) Start() {
	// fmt.Println(rand.Intn(max - min) + min)

}

func (q *Queue) Stop() {

}
