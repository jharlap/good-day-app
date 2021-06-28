package main

import (
	"fmt"
	"strconv"
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
