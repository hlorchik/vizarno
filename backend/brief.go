package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

//go:embed questions.json
var questionsJSON []byte

type question struct {
	Key, Label string
	Options    []string
	Optional   bool
}
type brief struct {
	Answers  map[string]string `json:"answers"`
	Audience string            `json:"audience"`
	Context  string            `json:"context"`
	Visual   string            `json:"visual"`
}

func makeBrief(r *http.Request) (*brief, error) {
	if r.FormValue("questionnaire") == "" {
		return nil, nil
	}
	var questions []question
	if err := json.Unmarshal(questionsJSON, &questions); err != nil {
		return nil, err
	}
	answers := map[string]string{}
	for _, q := range questions {
		v := strings.TrimSpace(r.FormValue(q.Key))
		if v == "" {
			continue
		}
		if len(q.Options) == 0 {
			if utf8.RuneCountInString(v) > 400 {
				return nil, fmt.Errorf("Ответ слишком длинный (до 400 символов): %s", q.Label)
			}
		} else if !slices.Contains(q.Options, v) {
			return nil, fmt.Errorf("Выберите ответ: %s", q.Label)
		}
		answers[q.Key] = v
	}
	audience := map[string]string{"Сам пользователь": "Покупатель выбирает для себя.", "Близкий человек": "Покупатель выбирает для другого человека.", "Закупщик для организации": "Покупатель выбирает для организации.", "Пока неизвестно": "Кто принимает решение, ещё предстоит уточнить."}[answers["buyer"]]
	context := answers["occasion"] + " · " + answers["channel"]
	for _, field := range [][2]string{{"task", "Задача покупателя"}, {"concern", "Вопрос до покупки"}, {"message", "Мысль для обложки"}, {"basis", "Основание со слов продавца"}} {
		if v := answers[field[0]]; v != "" {
			context += "\n" + field[1] + ": " + v
		}
	}
	advice := map[string]string{"Состав комплекта": "Покажите обложку и спросите, какие предметы войдут в заказ.", "Размер и совместимость": "Попросите назвать размер и объяснить, подойдёт ли товар для задачи.", "Внешний вид": "Попросите описать форму, цвет и фактуру по изображению.", "Цена": "Проверьте, понятно ли, какой товар и количество покупатель получит за указанную цену.", "Пока неизвестно": "Спросите покупателей, какую информацию они ищут первой."}[answers["priority"]]
	if answers["channel"] == "В мобильной выдаче" {
		advice += " Проведите проверку на миниатюре."
	}
	return &brief{answers, audience, context, answers["visual"] + ". " + advice}, nil
}
