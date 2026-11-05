package main

import (
	"fmt"
	"strings"
)

type layoutStep struct {
	Name        string   `json:"name"`
	Copy        []string `json:"copy"`
	Composition string   `json:"composition"`
	Purpose     string   `json:"purpose"`
	Acceptance  string   `json:"acceptance"`
}
type creativePlan struct {
	ReleaseNeeds []string     `json:"releaseNeeds,omitempty"`
	Materials    *materials   `json:"materials,omitempty"`
	Tasks        []finding    `json:"tasks,omitempty"`
	Title        string       `json:"title"`
	CardID       int64        `json:"cardId"`
	CoverID      int64        `json:"coverId,omitempty"`
	Source       string       `json:"source"`
	Context      string       `json:"context"`
	Steps        []layoutStep `json:"steps"`
	Evidence     []string     `json:"evidence"`
	Pending      []string     `json:"pending"`
	Export       []string     `json:"export"`
}

func planLayout(title string, id, version int64, profile productProfile, audit report) creativePlan {
	p := creativePlan{Title: title, CardID: id, CoverID: version, Source: "Обложка ещё не загружена. Запросите у продавца исходное фото этой конфигурации.", Context: "Сценарий покупки не указан. Для черновика используйте нейтральную сцену на рабочем столе. Перед публикацией согласуйте сценарий и главный вопрос покупателя."}
	if version != 0 {
		p.Source = fmt.Sprintf("Исходная обложка №%d · %d × %d px. Используйте её как референс внешности и цвета изделия.", version, audit.Width, audit.Height)
	}
	scene := "Покажите раскрытый ноутбук на чистой рабочей поверхности."
	if audit.Brief != nil {
		p.Context = audit.Brief.Audience + " " + audit.Brief.Context + ". " + audit.Brief.Visual
		switch audit.Brief.Answers["occasion"] {
		case "В подарок":
			scene = "Покажите момент вручения ноутбука. Подарочная упаковка допустима только как явно отделённый реквизит."
		case "Для работы":
			scene = "Покажите ноутбук на рабочем месте перед началом рабочего дня."
		case "Для учёбы":
			scene = "Покажите раскрытый ноутбук на учебном столе перед занятием."
		}
	}
	facts := map[string]string{}
	var specs []string
	for _, field := range [][2]string{{"model", "Модель"}, {"sku", "Артикул"}, {"cpu", "Процессор"}, {"ram", "ОЗУ"}, {"ssd", "SSD"}, {"os", "Операционная система"}, {"display", "Экран"}, {"graphics", "Графика"}, {"weight", "Вес"}, {"battery", "Автономность"}, {"charging", "Зарядка"}, {"ports", "Порты"}, {"keyboard", "Клавиатура"}, {"camera", "Камера"}, {"body", "Корпус"}, {"package", "Комплектация"}} {
		f := profile[field[0]]
		if f.Value == "" || f.Source == "" {
			p.Pending = append(p.Pending, field[1]+" — источник и значение отсутствуют.")
			continue
		}
		if f.Alternative != "" && !strings.EqualFold(f.Value, f.Alternative) {
			p.Pending = append(p.Pending, fmt.Sprintf("%s — согласовать до публикации: «%s» (%s) / «%s» (%s).", field[1], f.Value, f.Source, f.Alternative, f.OtherSource))
			continue
		}
		if field[0] == "os" && contradictingOS(audit.Materials, f) != "" {
			p.Pending = append(p.Pending, "Операционная система: профиль расходится с описанием. Подтвердите комплектацию до публикации.")
			continue
		}
		facts[field[0]] = f.Value
		p.Evidence = append(p.Evidence, fmt.Sprintf("%s — %s (%s)", field[1], f.Value, f.Source))
		if field[0] == "cpu" || field[0] == "ram" || field[0] == "ssd" || field[0] == "graphics" {
			specs = append(specs, field[1]+" · "+f.Value)
		}
	}
	cover := layoutStep{
		Name: "01 · Обложка", Purpose: "За 1–2 секунды связать точную модель с ситуацией покупателя и призвать к покупке.",
		Composition: scene + " Сохраните пропорции изделия. Подберите масштаб так, чтобы экран и клавиатура различались в миниатюре.",
		Acceptance:  "В миниатюре 120 × 150 px различимы товар, фраза «Тот самый» и призыв «Покупай!». Ноутбук занимает 45–65% полезной площади.",
	}
	if model := facts["model"]; model != "" {
		anchor := "Тот самый ноутбук"
		if audit.Brief != nil {
			switch audit.Brief.Answers["occasion"] {
			case "Для учёбы":
				anchor = "Тот самый ноутбук для учёбы"
			case "Для работы":
				anchor = "Тот самый ноутбук для работы"
			case "В подарок":
				anchor = "Тот самый ноутбук в подарок"
			}
		}
		cover.Copy = []string{anchor, model, "Покупай!"}
		cover.Composition += " Если используете подпись модели, разместите её на свободном поле и сравните читаемость вариантов."
		cover.Acceptance += " Если добавлена подпись, она читается и совпадает с подтверждённым обозначением модели."
	}
	p.ReleaseNeeds = generationNeeds(audit.Brief, facts)
	p.Steps = append(p.Steps, cover)

	// The replacement methodology requires eight distinct jobs in a fixed emotional-to-rational sequence.
	if audit.Brief != nil {
		a := audit.Brief.Answers
		if value := facts[benefitField(a["benefitField"])]; value != "" && a["advantage"] != "" && a["benefit"] != "" && a["proof"] != "" {
			p.Steps = append(p.Steps, layoutStep{Name: "02 · Главная выгода", Copy: []string{a["benefit"], a["benefitField"] + " · " + value, a["advantage"]}, Purpose: "Раскрыть центральную идею серии через одну цепочку «свойство → преимущество → выгода».", Composition: "Один главный тезис, один визуальный центр и не более двух доказательств. " + scene, Acceptance: "За 1–2 секунды понятна выгода для указанной ситуации; факт совпадает с профилем."})
			p.Evidence = append(p.Evidence, "Основание для преимущества: "+a["proof"])
		}
		p.Steps = append(p.Steps, layoutStep{Name: "03 · Товар в ситуации", Copy: []string{a["task"], a["motive"]}, Purpose: "Помочь покупателю узнать свою ситуацию и положительный образ использования.", Composition: scene + " Используйте мягкий естественный свет и ощущение порядка. Реквизит явно отделён от комплекта.", Acceptance: "Ситуация понятна без подробного текста; конструкция и цвет ноутбука не изменены."})
	}
	if len(specs) > 0 {
		p.Steps = append(p.Steps, layoutStep{"04 · Производительность", append([]string{"Подтверждённая конфигурация"}, specs...), "Слева крупный план ноутбука, справа — подтверждённые параметры. Не добавляйте измерения скорости и обещания результата.", "Дать рациональные основания для учебной задачи.", "На слайде только подтверждённые значения; один параметр не выдаётся за гарантию производительности."})
	}
	if display := facts["display"]; display != "" {
		p.Steps = append(p.Steps, layoutStep{"05 · Экран", []string{"Экран", display}, "Покажите фронтальный вид экрана без выдуманного интерфейса. Оставьте свободную область под один короткий тезис.", "Показать подтверждённые свойства экрана отдельно от остальных параметров.", "Диагональ, разрешение, яркость и частота указаны только в объёме подтверждённого источника."})
	}
	mobility := []string{"Повседневное использование"}
	for _, key := range []string{"weight", "battery", "charging"} {
		if facts[key] != "" {
			mobility = append(mobility, facts[key])
		}
	}
	if len(mobility) == 1 {
		mobility = append(mobility, "Данные о весе, автономности и зарядке нужно подтвердить")
	}
	p.Steps = append(p.Steps, layoutStep{"06 · Мобильность и повседневность", mobility, "Покажите спокойный повседневный сценарий. Не изображайте переноску или зарядное устройство как часть комплекта без подтверждения.", "Ответить на вопрос об удобстве повседневного использования.", "Неподтверждённые вес, автономность и зарядка не превращены в обещания."})
	details := []string{"Детали и подключения"}
	for _, key := range []string{"ports", "keyboard", "camera", "body"} {
		if facts[key] != "" {
			details = append(details, facts[key])
		}
	}
	if len(details) == 1 {
		details = append(details, "Используйте только видимые детали исходных фотографий")
	}
	p.Steps = append(p.Steps, layoutStep{"07 · Детали и подключения", details, "Используйте макродеталь из исходной фотографии. Не дорисовывайте порты, клавиатуру, камеру, шарниры или фактуру.", "Показать наблюдаемые конструктивные детали без догадок.", "Каждая показанная деталь сверяется с исходником; спорная сторона товара не показана как факт."})
	rational := []string{"Точная конфигурация"}
	for _, key := range []string{"model", "sku", "cpu", "ram", "ssd", "os", "package"} {
		if facts[key] != "" {
			rational = append(rational, facts[key])
		}
	}
	if os := facts["os"]; os != "" {
		lines := []string{"Операционная система · " + os}
		if strings.EqualFold(os, "без ОС") || strings.EqualFold(os, "noOS") {
			lines = []string{"Без ОС", "Перед началом работы потребуется установить операционную систему."}
		}
		rational = append(rational, lines...)
	}
	p.Steps = append(p.Steps, layoutStep{"08 · Рациональное подтверждение", rational, "Структурированный технический блок с точной конфигурацией, комплектацией и ограничениями. Используйте фронтальный ракурс без выдуманных деталей.", "Дать исследователю проверяемую сводку и снять критические препятствия.", "Все цифры имеют источник; ограничения и отсутствие ОС заметны; данные других конфигураций не смешаны."})
	if audit.Brief != nil {
		switch audit.Brief.Answers["priority"] {
		case "Состав комплекта":
			p.Pending = append(p.Pending, "Запросите точный состав поставки. Для покупателя важен комплект: после подтверждения покажите входящие предметы с подписями, реквизит уберите из этого кадра.")
		case "Цена":
			p.Pending = append(p.Pending, "Запросите действующую цену и условия предложения. Покажите рядом, за какую конфигурацию платит покупатель.")
		case "Внешний вид":
			p.Steps[0].Composition += " Добавьте к основному фото один согласованный крупный план фактуры или отделки."
		case "Размер и совместимость":
			p.Pending = append(p.Pending, "Согласуйте размер и нужный покупателю способ подключения. Покажите реальный разъём или габарит с точной подписью после сверки.")
		}
	}
	if len(facts) == 0 {
		p.Pending = append(p.Pending, "Сейчас готов только композиционный черновик. Для содержательного слайда продавцу нужно подтвердить хотя бы одно свойство, его пользу и ситуацию использования.")
	}
	p.Export = []string{"Сдайте PNG каждого согласованного слайда и редактируемый исходник. Сохраните условные предложения отдельно от текста для публикации.", "Сопоставьте макет с исходной версией и проверьте каждый критерий приёмки. Не добавляйте цены, скидки, отзывы и обещания без источника."}
	for _, c := range audit.Checks {
		if !c.OK {
			p.Export = append(p.Export, c.Label+". "+c.Note)
		}
	}
	return p
}
func (p creativePlan) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ВИЗАРНО · ЗАДАНИЕ ДИЗАЙНЕРУ\n%s · карточка №%d\n%s\n\nСценарий\n%s\n", p.Title, p.CardID, p.Source, p.Context)
	var fixes []string
	pending := append([]string(nil), p.Pending...)
	for _, task := range p.Tasks {
		if task.Status == "unknown" {
			pending = append(pending, task.Source+" — "+task.Action)
			continue
		}
		fixes = append(fixes, fmt.Sprintf("• %s · %s\n  Основание: %s\n  Действие: %s\n  Приёмка: %s", task.Source, task.Rule, task.Evidence, task.Action, task.Verify))
	}
	if len(fixes) > 0 {
		fmt.Fprintf(&b, "\nВыбранные правки\n%s\n", strings.Join(fixes, "\n"))
	}
	for _, s := range p.Steps {
		fmt.Fprintf(&b, "\n%s\nЗадача: %s\n", s.Name, s.Purpose)
		if len(s.Copy) > 0 {
			fmt.Fprintf(&b, "Текст для макета:\n%s\n", strings.Join(s.Copy, "\n"))
		} else {
			b.WriteString("Текст для макета пока не согласован.\n")
		}
		fmt.Fprintf(&b, "Композиция: %s\nПриёмка: %s\n", s.Composition, s.Acceptance)
	}
	for _, section := range []struct {
		name  string
		lines []string
	}{{"Основания для текста", p.Evidence}, {"Уточнить у продавца", pending}, {"Передача результата", p.Export}} {
		if len(section.lines) > 0 {
			fmt.Fprintf(&b, "\n%s\n• %s\n", section.name, strings.Join(section.lines, "\n• "))
		}
	}
	return b.String()
}
