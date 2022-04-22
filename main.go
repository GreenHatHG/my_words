package main

import (
	"fmt"
	"github.com/desertbit/grumble"
	"github.com/fatih/color"
	"github.com/kamva/mgm/v3"
	"github.com/schollz/progressbar/v3"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"time"
)

const (
	Word     = "word"
	Sentence = "sentence"
	Remark   = "remark"
)

//例句以及备注
type WordSentence struct {
	Sentence string
	Remark   string
}

//一条单词记录
type Record struct {
	// DefaultModel adds _id, created_at and updated_at fields to the Model
	mgm.DefaultModel `bson:",inline"`
	Word             string         `json:"word" bson:"word"`
	WordSentence     []WordSentence `json:"word_sentence" bson:"word_sentence"`
	NumReview        int64          `json:"num_review" bson:"num_review"`
}

func init() {
	done := make(chan struct{})
	go func() {
		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetWidth(10),
			progressbar.OptionSetDescription("开始连接数据库..."),
			//progressbar.OptionShowIts(),
			//progressbar.OptionShowCount(),
			progressbar.OptionSpinnerType(1),
		)
		for {
			select {
			case <-done:
				return
			default:
				time.Sleep(500 * time.Millisecond)
				_ = bar.Add(1)
			}
		}
	}()

	uri := "mongodb+srv://words:uoibcRPDTMx2kegm@cluster0.kb0tl.mongodb.net"
	mgmOptions := options.Client().ApplyURI(uri).SetRetryWrites(true).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
	if err := mgm.SetDefaultConfig(nil, "my_words", mgmOptions); err != nil {
		panic(err)
	}
	_, client, _, _ := mgm.DefaultConfigs()
	if err := client.Ping(nil, nil); err != nil {
		panic(err)
	}

	fmt.Println("\n连接数据库成功...")
	done <- struct{}{}
}

func Success(format string, a ...interface{}) {
	color.Green(format, a...)
}

func NewRecord(word, sentence, remark string) *Record {
	return &Record{
		Word:         word,
		WordSentence: []WordSentence{{sentence, remark}},
		NumReview:    0,
	}
}

func AddRecord(word, sentence, remark string) {
	record := NewRecord(word, sentence, remark)
	if err := mgm.Coll(record).Create(record); err != nil {
		panic(err)
	}
}

func main() {
	var app = grumble.New(&grumble.Config{
		Name:        "my_words",
		Description: "A4背单词CLI",
	})
	app.AddCommand(&grumble.Command{
		Name:    "add",
		Help:    "添加新的单词",
		Aliases: []string{"a"},

		Args: func(a *grumble.Args) {
			a.String(Word, "英文单词/短语")
			a.String(Sentence, "例句")
			a.String(Remark, "备注")
		},

		Run: func(c *grumble.Context) error {
			AddRecord(c.Args.String(Word), c.Args.String(Sentence), c.Args.String(Remark))
			Success("添加成功")
			return nil
		},
	})

	grumble.Main(app)
}
