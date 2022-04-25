package main

import (
	"fmt"
	"github.com/desertbit/grumble"
	"github.com/fatih/color"
	"github.com/kamva/mgm/v3"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"gopkg.in/AlecAivazis/survey.v1"
	"math/rand"
	"os"
	"strconv"
	"strings"
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
	ReviewTimes      []time.Time    `json:"review_times" bson:"review_times"`
}

func InitDB(password, host string) {
	done := make(chan struct{}, 1)
	go func() {
		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetWidth(10),
			progressbar.OptionSetDescription("开始连接数据库..."),
			progressbar.OptionShowCount(),
			progressbar.OptionSpinnerType(1),
		)
		t := time.NewTimer(20 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-done:
				return
			case <-t.C:
				panic("\n连接数据库超时，请重试...")
				return
			default:
				time.Sleep(1 * time.Second)
				_ = bar.Add(1)
			}
		}
	}()

	uri := fmt.Sprintf("mongodb+srv://words:%s@%s", password, host)
	mgmOptions := options.Client().ApplyURI(uri).SetRetryWrites(true).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
	if err := mgm.SetDefaultConfig(nil, "my_words", mgmOptions); err != nil {
		panic(err)
	}
	_, client, _, _ := mgm.DefaultConfigs()
	if err := client.Ping(nil, nil); err != nil {
		panic(err)
	}

	done <- struct{}{}
	time.Sleep(1 * time.Second)
}

func Success(format string, a ...interface{}) {
	color.Green(format, a...)
}

func Failure(format string, a ...interface{}) {
	color.Red(format, a...)
}

func Info(format string, a ...interface{}) {
	color.Blue(format, a...)
}

func NewRecord(word, sentence, remark string) *Record {
	now := time.Now().UTC()

	reviewsInterval := []int{0, 1, 2, 4, 7, 15}
	var reviewTimes []time.Time

	for _, interval := range reviewsInterval {
		temp := now.Add(time.Duration(interval) * 24 * time.Hour)
		reviewDay := time.Date(temp.Year(), temp.Month(), temp.Day(), 0, 0, 0, 0, now.Location())
		reviewTimes = append(reviewTimes, reviewDay)
	}

	return &Record{
		Word:         word,
		WordSentence: []WordSentence{{sentence, remark}},
		NumReview:    0,
		ReviewTimes:  reviewTimes,
	}
}

func PrintWordTable(records []*Record) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Word", "Sentence", "Remark", "NumReview"})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	for _, record := range records {
		for _, item := range record.WordSentence {
			table.Append([]string{record.Word, item.Sentence, item.Remark, strconv.FormatInt(record.NumReview, 10)})
		}
	}
	table.Render()
}

func AddRecord(word, sentence, remark string) {
	word = strings.Trim(word, "")
	sentence = strings.Trim(sentence, "")
	remark = strings.Trim(remark, "")

	record := &Record{}
	err := mgm.Coll(record).First(bson.M{"word": word}, record)
	if err != nil && err != mongo.ErrNoDocuments {
		panic(err)
	}

	//新增
	if err == mongo.ErrNoDocuments {
		record = NewRecord(word, sentence, remark)
		if err := mgm.Coll(record).Create(record); err != nil {
			panic(err)
		}
		Success("添加成功")
		return
	}

	//检查是否已经有该sentence
	for _, item := range record.WordSentence {
		if item.Sentence == sentence {
			Failure("该例句已经存在")
			PrintWordTable([]*Record{record})
			return
		}
	}

	//合并sentence到word
	record.WordSentence = append(record.WordSentence, WordSentence{
		Sentence: sentence,
		Remark:   remark,
	})
	if err := mgm.Coll(record).Update(record); err != nil {
		panic(err)
	}
	Success(fmt.Sprintf("合并sentence到%s成功", word))
}

func DeleteRecord(word string) {
	record := &Record{Word: word}
	if err := mgm.Coll(record).Delete(record); err != nil {
		panic(err)
	}
	Success("删除成功")
}

func AllRecord() {
	var records []*Record
	if err := mgm.Coll(&Record{}).SimpleFind(&records, bson.M{}); err != nil {
		panic(err)
	}
	PrintWordTable(records)
}

func FindRecord(word string) {
	record := &Record{}
	err := mgm.Coll(record).First(bson.M{"word": word}, record)
	if err != nil && err != mongo.ErrNoDocuments {
		panic(err)
	}
	PrintWordTable([]*Record{record})
}

func TruncateRecord() {
	if _, err := mgm.Coll(&Record{}).DeleteMany(nil, bson.M{}); err != nil {
		panic(err)
	}
	Success("删除成功")
}

func ReviewWords() {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 59, now.Location())
	query := bson.M{"review_times": bson.M{"$gte": start, "$lte": end}}

	var records []*Record
	if err := mgm.Coll(&Record{}).SimpleFind(&records, query); err != nil {
		panic(err)
	}

	if len(records) == 0 {
		fmt.Println("==暂无需要复习的单词==")
		return
	}

	//洗牌
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(records), func(i, j int) { records[i], records[j] = records[j], records[i] })

	var operations []mongo.WriteModel

	for i, record := range records {
		for _, item := range record.WordSentence {
			ok := false
			message := fmt.Sprintf("[%s] [%s]", record.Word, item.Sentence)
			if err := survey.AskOne(&survey.Confirm{Message: message, Default: true}, &ok, nil); err != nil {
				panic(err)
			}
		}
		records[i].NumReview++
		operation := mongo.NewUpdateOneModel()
		operation.SetFilter(bson.M{"word": records[i].Word})
		operation.SetUpdate(bson.M{"$set": bson.M{"num_review": records[i].NumReview}})
		operations = append(operations, operation)
	}
	if _, err := mgm.Coll(&Record{}).BulkWrite(nil, operations); err != nil {
		panic(err)
	}
}

func main() {
	var app = grumble.New(&grumble.Config{
		Name:        "my_words",
		Description: "A4背单词CLI",
		Flags: func(f *grumble.Flags) {
			f.String("p", "password", "", "数据库密码")
			f.String("u", "host", "", "数据库地址")
		},
	})

	app.OnInit(func(a *grumble.App, flags grumble.FlagMap) error {
		InitDB(flags.String("password"), flags.String("host"))
		return nil
	})
	app.AddCommand(&grumble.Command{
		Name: "add",
		Help: "添加新的单词",

		Args: func(a *grumble.Args) {
			a.String(Word, "英文单词/短语")
			a.String(Sentence, "例句")
			a.String(Remark, "备注")
		},

		Flags: func(f *grumble.Flags) {
			f.Bool("d", "direct", false, "添加单词后关闭随机复习")
		},

		Run: func(c *grumble.Context) error {
			AddRecord(c.Args.String(Word), c.Args.String(Sentence), c.Args.String(Remark))
			if !c.Flags.Bool("direct") {
				Info("开始随机复习单词")
				ReviewWords()
			}
			return nil
		},
	})
	app.AddCommand(&grumble.Command{
		Name: "del",
		Help: "删除单词",

		Args: func(a *grumble.Args) {
			a.String(Word, "英文单词/短语")
		},

		Run: func(c *grumble.Context) error {
			DeleteRecord(c.Args.String(Word))
			return nil
		},
	})
	app.AddCommand(&grumble.Command{
		Name: "all",
		Help: "显示所有单词",

		Run: func(c *grumble.Context) error {
			AllRecord()
			return nil
		},
	})
	app.AddCommand(&grumble.Command{
		Name: "show",
		Help: "查询一个单词",

		Args: func(a *grumble.Args) {
			a.String(Word, "英文单词/短语")
		},

		Run: func(c *grumble.Context) error {
			FindRecord(c.Args.String(Word))
			return nil
		},
	})
	app.AddCommand(&grumble.Command{
		Name: "truncate",
		Help: "删除所有单词",

		Run: func(c *grumble.Context) error {
			TruncateRecord()
			return nil
		},
	})
	app.AddCommand(&grumble.Command{
		Name: "review",
		Help: "艾宾浩斯复习单词",

		Run: func(c *grumble.Context) error {
			ReviewWords()
			return nil
		},
	})
	grumble.Main(app)
}
