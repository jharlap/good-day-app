package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"

	"github.com/slack-go/slack"
)

type Reflection struct {
	TeamID                string             `db:"team_id"`
	UserID                string             `db:"user_id"`
	Date                  string             `db:"date"`
	WorkDayQuality        NumberPrefixedEnum `db:"work_day_quality"`
	WorkOtherPeopleAmount NumberPrefixedEnum `db:"work_other_people_amount"`
	HelpOtherPeopleAmount NumberPrefixedEnum `db:"help_other_people_amount"`
	InterruptedAmount     NumberPrefixedEnum `db:"interrupted_amount"`
	ProgressGoalsAmount   NumberPrefixedEnum `db:"progress_goals_amount"`
	QualityWorkAmount     NumberPrefixedEnum `db:"quality_work_amount"`
	LotOfWorkAmount       NumberPrefixedEnum `db:"lot_of_work_amount"`
	WorkDayFeeling        NumberPrefixedEnum `db:"work_day_feeling"`
	StressfulAmount       NumberPrefixedEnum `db:"stressful_amount"`
	BreaksAmount          NumberPrefixedEnum `db:"breaks_amount"`
	MeetingNumber         NumberPrefixedEnum `db:"meeting_number"`
	MostProductiveTime    NumberPrefixedEnum `db:"most_productive_time"`
	LeastProductiveTime   NumberPrefixedEnum `db:"least_productive_time"`
}

func (r Reflection) String() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "Date (UTC): %s\n", r.Date)
	for _, q := range Questions {
		fmt.Fprintf(buf, "%s: *%s*\n", q.Text, q.Options.ValueFor(r.ValueForQuestion(q.Field)))
	}
	return buf.String()
}

func (r Reflection) ValueForQuestion(field string) string {
	rv := reflect.ValueOf(r)
	for i := 0; i < rv.NumField(); i++ {
		if t, ok := rv.Type().Field(i).Tag.Lookup("db"); ok && t == field {
			return rv.Field(i).String()
		}
	}
	return ""
}

type Question struct {
	Text    string
	Field   string
	Options OptionSet
}

func (q Question) SlackBlock() *slack.InputBlock {
	return slack.NewInputBlock(
		q.Field,
		slack.NewTextBlockObject(slack.PlainTextType, q.Text, false, false),
		q.Options.SlackElement(),
	)
}

type OptionSet struct {
	Placeholder string
	Options     []Option
}

func (o OptionSet) SlackElement() *slack.SelectBlockElement {
	var opts []*slack.OptionBlockObject
	for _, opt := range o.Options {
		opts = append(opts, opt.SlackOption())
	}
	return slack.NewOptionsSelectBlockElement(
		"static_select",
		slack.NewTextBlockObject(slack.PlainTextType, o.Placeholder, false, false),
		"select",
		opts...,
	)
}

func (o OptionSet) ValueFor(code string) string {
	for _, opt := range o.Options {
		if opt.Code == code {
			return opt.Text
		}
	}
	return ""
}

type Option struct {
	Text string
	Code string
}

func (o Option) SlackOption() *slack.OptionBlockObject {
	return slack.NewOptionBlockObject(
		o.Code,
		slack.NewTextBlockObject(slack.PlainTextType, o.Text, false, false),
		nil,
	)
}

var (
	QualityOptions = OptionSet{
		Placeholder: "Pick the closest",
		Options: []Option{
			{Text: "Terrible", Code: "0-terrible"},
			{Text: "Bad", Code: "1-bad"},
			{Text: "OK", Code: "2-ok"},
			{Text: "Good", Code: "3-good"},
			{Text: "Awesome", Code: "4-awesome"},
		},
	}

	AmountOfDayOptions = OptionSet{
		Placeholder: "How much of the day",
		Options: []Option{
			{Text: "None of the day", Code: "0-none"},
			{Text: "A little of the day", Code: "1-little"},
			{Text: "Some of the day", Code: "2-some"},
			{Text: "Much of the day", Code: "3-much"},
			{Text: "Most or all of the day", Code: "4-most"},
		},
	}

	FeelingOptions = OptionSet{
		Placeholder: "Pick the closest",
		Options: []Option{
			{Text: "Tense or nervous", Code: "0-tense"},
			{Text: "Stressed or upset", Code: "1-stress"},
			{Text: "Sad or depressed", Code: "2-sad"},
			{Text: "Bored", Code: "3-bored"},
			{Text: "Calm or relaxed", Code: "4-calm"},
			{Text: "Serene or content", Code: "5-serene"},
			{Text: "Happy or elated", Code: "6-happy"},
			{Text: "Excited or alert", Code: "7-excited"},
		},
	}

	NumberOptions = OptionSet{
		Placeholder: "How many",
		Options: []Option{
			{Text: "0", Code: "0-none"},
			{Text: "1", Code: "1-one"},
			{Text: "2", Code: "2-two"},
			{Text: "3-4", Code: "3-few"},
			{Text: "5 or more", Code: "4-many"},
		},
	}

	TimeOptions = OptionSet{
		Placeholder: "Which part of the day",
		Options: []Option{
			{Text: "In the morning (9:00 – 11:00)", Code: "0-morning"},
			{Text: "Mid-day (11:00 – 13:00)", Code: "1-midday"},
			{Text: "In the early afternoon (13:00 – 15:00)", Code: "2-earlyAft"},
			{Text: "In the late afternoon (15:00 – 17:00)", Code: "3-lateAft"},
			{Text: "Outside typical work hours", Code: "4-nonwork"},
			{Text: "Equally throughout the day", Code: "5-equally"},
		},
	}

	Questions []Question = []Question{
		{Text: "How was your work day?", Field: "work_day_quality", Options: QualityOptions},
		{Text: "I worked with other people", Field: "work_other_people_amount", Options: AmountOfDayOptions},
		{Text: "I helped other people", Field: "help_other_people_amount", Options: AmountOfDayOptions},
		{Text: "My work was interrupted", Field: "interrupted_amount", Options: AmountOfDayOptions},
		{Text: "I made progress toward my goals", Field: "progress_goals_amount", Options: AmountOfDayOptions},
		{Text: "I did high-quality work", Field: "quality_work_amount", Options: AmountOfDayOptions},
		{Text: "I did a lot of work", Field: "lot_of_work_amount", Options: AmountOfDayOptions},
		{Text: "Which best describes how you feel about your work day?", Field: "work_day_feeling", Options: FeelingOptions},
		{Text: "My day was stressful", Field: "stressful_amount", Options: AmountOfDayOptions},
		{Text: "I took breaks today", Field: "breaks_amount", Options: AmountOfDayOptions},
		{Text: "How many meetings did you have today?", Field: "meeting_number", Options: NumberOptions},
		{Text: "Today, I felt most productive", Field: "most_productive_time", Options: TimeOptions},
		{Text: "Today, I felt least productive", Field: "least_productive_time", Options: TimeOptions},
	}
)

type NumberPrefixedEnum string

func (e *NumberPrefixedEnum) Scan(src interface{}) error {
	v, ok := src.(string)
	if !ok {
		return fmt.Errorf("error scanning quality %+v", src)
	}

	*e = NumberPrefixedEnum(v)
	return nil
}

func (e *NumberPrefixedEnum) Value() interface{} {
	if e == nil {
		return nil
	}

	return string(*e)
}

func (e *NumberPrefixedEnum) IntVal() int {
	if e == nil || len(string(*e)) == 0 {
		return -1
	}

	i, err := strconv.Atoi(string(*e)[0:1])
	if err != nil {
		return -1
	}
	return i
}
