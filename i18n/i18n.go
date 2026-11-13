// Package i18n renders a measurement contract into human languages.
//
// The contract is one source of truth. Every language is generated from it, so
// a number can never drift between translations: there is only one number,
// and the renderings differ only in the words around it.
//
// Four languages ship with the tool: English, Russian, Spanish and Chinese.
// A locale that is not listed falls back to English rather than failing, so
// an unknown code in a contract is a warning, not a crash.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Locale codes shipped with the tool.
const (
	English  = "en"
	Russian  = "ru"
	Spanish  = "es"
	Chinese  = "zh"
)

// Locales returns the shipped locale codes in a stable order.
func Locales() []string {
	return []string{English, Russian, Spanish, Chinese}
}

// strings holds the translated labels. The key set is closed: every label the
// renderer can ask for must exist for every shipped locale, and the package
// test asserts that.
var table = map[string]map[string]string{
	English: {
		"title":              "MEASUREMENT PROTOCOL",
		"contract":           "CONTRACT",
		"pin":                "CONTRACT PIN",
		"candidate":          "CANDIDATE",
		"baseline":           "BASELINE",
		"metric":             "METRIC",
		"judge":              "JUDGE",
		"selection":          "SELECTION SPLIT",
		"heldout":            "HELD-OUT SPLIT",
		"candidates":         "CANDIDATES IN ROUND",
		"corr":               "CANDIDATE CORRELATION",
		"alpha":              "SIGNIFICANCE",
		"min_gain":           "MINIMUM GAIN",
		"power":              "TARGET POWER",
		"note":               "NOTE",
		"verdict":            "VERDICT",
		"verdict_proven":     "PROVEN",
		"verdict_not_proven": "NOT PROVEN",
		"verdict_conflicting": "CONFLICTING",
		"claimed_gain":       "CLAIMED GAIN",
		"selection_inflation": "EXPECTED SELECTION INFLATION",
		"corrected_claim":    "GAIN AFTER SELECTION CORRECTION",
		"heldout_gain":       "HELD-OUT GAIN",
		"interval":           "HELD-OUT INTERVAL",
		"lower_bound":        "LOWER BOUND",
		"required_heldout":   "HELD-OUT ITEMS NEEDED",
		"sigma":              "PER-ITEM SPREAD",
		"sigma_source":       "SPREAD SOURCE",
		"selected_on":        "used to pick the candidate",
		"never_used":         "never used to pick the candidate",
		"items":              "items",
		"expected_max":       "expected maximum",
		"sigma_estimated":    "estimated from the held-out split",
		"sigma_worst":        "worst case for a binary metric",
		"source":             "SOURCE",
	},
	Russian: {
		"title":              "ПРОТОКОЛ ИЗМЕРЕНИЯ",
		"contract":           "КОНТРАКТ",
		"pin":                "ПИН КОНТРАКТА",
		"candidate":          "КАНДИДАТ",
		"baseline":           "БАЗОВАЯ СИСТЕМА",
		"metric":             "МЕТРИКА",
		"judge":              "СПОСОБ ОЦЕНКИ",
		"selection":          "ВЫБОРКА ОТБОРА",
		"heldout":            "ОТЛОЖЕННАЯ ВЫБОРКА",
		"candidates":         "КАНДИДАТОВ В РАУНДЕ",
		"corr":               "КОРРЕЛЯЦИЯ КАНДИДАТОВ",
		"alpha":              "УРОВЕНЬ ЗНАЧИМОСТИ",
		"min_gain":           "МИНИМАЛЬНЫЙ ПРИРОСТ",
		"power":              "ЦЕЛЕВАЯ МОЩНОСТЬ",
		"note":               "ЗАМЕТКА",
		"verdict":            "ВЕРДИКТ",
		"verdict_proven":     "ПОДТВЕРЖДЕНО",
		"verdict_not_proven": "НЕ ПОДТВЕРЖДЕНО",
		"verdict_conflicting": "ПРОТИВОРЕЧИЕ",
		"claimed_gain":       "ЗАЯВЛЕННЫЙ ПРИРОСТ",
		"selection_inflation": "ОЖИДАЕМОЕ ЗАВЫШЕНИЕ ОТ ОТБОРА",
		"corrected_claim":    "ПРИРОСТ ПОСЛЕ КОРРЕКЦИИ ОТБОРА",
		"heldout_gain":       "ПРИРОСТ НА ОТЛОЖЕННОЙ ВЫБОРКЕ",
		"interval":           "ИНТЕРВАЛ НА ОТЛОЖЕННОЙ ВЫБОРКЕ",
		"lower_bound":        "НИЖНЯЯ ГРАНИЦА",
		"required_heldout":   "НУЖНО ЭЛЕМЕНТОВ ОТЛОЖЕННОЙ ВЫБОРКИ",
		"sigma":              "РАЗБРОС НА ЭЛЕМЕНТ",
		"sigma_source":       "ИСТОЧНИК РАЗБРОСА",
		"selected_on":        "использована для выбора кандидата",
		"never_used":         "не участвовала в выборе кандидата",
		"items":              "элементов",
		"expected_max":       "ожидаемый максимум",
		"sigma_estimated":    "оценён по отложенной выборке",
		"sigma_worst":        "худший случай для бинарной метрики",
		"source":             "ИСТОЧНИК",
	},
	Spanish: {
		"title":              "PROTOCOLO DE MEDICIÓN",
		"contract":           "CONTRATO",
		"pin":                "HUELLA DEL CONTRATO",
		"candidate":          "CANDIDATO",
		"baseline":           "LÍNEA BASE",
		"metric":             "MÉTRICA",
		"judge":              "JUEZ",
		"selection":          "MUESTRA DE SELECCIÓN",
		"heldout":            "MUESTRA RESERVADA",
		"candidates":         "CANDIDATOS EN LA RONDA",
		"corr":               "CORRELACIÓN ENTRE CANDIDATOS",
		"alpha":              "NIVEL DE SIGNIFICANCIA",
		"min_gain":           "GANANCIA MÍNIMA",
		"power":              "POTENCIA OBJETIVO",
		"note":               "NOTA",
		"verdict":            "VEREDICTO",
		"verdict_proven":     "PROBADO",
		"verdict_not_proven": "NO PROBADO",
		"verdict_conflicting": "CONTRADICTORIO",
		"claimed_gain":       "GANANCIA DECLARADA",
		"selection_inflation": "INFLACIÓN ESPERADA POR SELECCIÓN",
		"corrected_claim":    "GANANCIA TRAS LA CORRECCIÓN",
		"heldout_gain":       "GANANCIA EN LA MUESTRA RESERVADA",
		"interval":           "INTERVALO EN LA MUESTRA RESERVADA",
		"lower_bound":        "LÍMITE INFERIOR",
		"required_heldout":   "ELEMENTOS RESERVADOS NECESARIOS",
		"sigma":              "DISPERSIÓN POR ELEMENTO",
		"sigma_source":       "ORIGEN DE LA DISPERSIÓN",
		"selected_on":        "usada para elegir el candidato",
		"never_used":         "nunca usada para elegir el candidato",
		"items":              "elementos",
		"expected_max":       "máximo esperado",
		"sigma_estimated":    "estimada en la muestra reservada",
		"sigma_worst":        "peor caso para una métrica binaria",
		"source":             "FUENTE",
	},
	Chinese: {
		"title":              "测量协议",
		"contract":           "契约",
		"pin":                "契约指纹",
		"candidate":          "候选",
		"baseline":           "基线",
		"metric":             "指标",
		"judge":              "判定方式",
		"selection":          "选择集",
		"heldout":            "留出集",
		"candidates":         "本轮候选数",
		"corr":               "候选相关性",
		"alpha":              "显著性水平",
		"min_gain":           "最小增益",
		"power":              "目标功效",
		"note":               "备注",
		"verdict":            "结论",
		"verdict_proven":     "已证实",
		"verdict_not_proven": "未证实",
		"verdict_conflicting": "存在冲突",
		"claimed_gain":       "声称增益",
		"selection_inflation": "选择带来的预期高估",
		"corrected_claim":    "校正选择偏差后的增益",
		"heldout_gain":       "留出集增益",
		"interval":           "留出集区间",
		"lower_bound":        "下界",
		"required_heldout":   "所需留出样本数",
		"sigma":              "单样本离散度",
		"sigma_source":       "离散度来源",
		"selected_on":        "用于挑选候选",
		"never_used":         "从未用于挑选候选",
		"items":              "个样本",
		"expected_max":       "期望最大值",
		"sigma_estimated":    "由留出集估计",
		"sigma_worst":        "二值指标的最坏情况",
		"source":             "来源",
	},
}

// reasons holds the one-sentence explanations of a verdict. They are kept
// apart from the label table because they carry values.
var reasons = map[string]map[string]string{
	English: {
		"proven":     "the held-out lower bound %s clears the minimum gain %s",
		"not_proven": "the held-out lower bound %s does not clear the minimum gain %s",
		"conflicting": "the selection split reports %s, but the held-out lower bound %s does not clear %s",
	},
	Russian: {
		"proven":     "нижняя граница отложенной выборки %s превышает минимальный прирост %s",
		"not_proven": "нижняя граница отложенной выборки %s не превышает минимальный прирост %s",
		"conflicting": "выборка отбора показывает %s, но нижняя граница отложенной выборки %s не превышает %s",
	},
	Spanish: {
		"proven":     "el límite inferior de la muestra reservada %s supera la ganancia mínima %s",
		"not_proven": "el límite inferior de la muestra reservada %s no supera la ganancia mínima %s",
		"conflicting": "la muestra de selección declara %s, pero el límite inferior de la muestra reservada %s no supera %s",
	},
	Chinese: {
		"proven":     "留出集下界 %s 已超过最小增益 %s",
		"not_proven": "留出集下界 %s 未达到最小增益 %s",
		"conflicting": "选择集显示 %s，但留出集下界 %s 未达到 %s",
	},
}

// Reason returns the translated explanation of a verdict with the values
// already substituted.
func Reason(locale, verdict string, args ...string) string {
	tmpl, ok := reasons[locale][verdict]
	if !ok {
		tmpl = reasons[English][verdict]
	}
	if tmpl == "" {
		return ""
	}
	anyArgs := make([]any, 0, len(args))
	for _, a := range args {
		anyArgs = append(anyArgs, a)
	}
	return fmt.Sprintf(tmpl, anyArgs...)
}

// T returns the translation of key in the given locale.
//
// An unknown locale falls back to English. A missing key falls back to the
// English string and finally to the key itself, so a renderer never panics on
// a label it does not know.
func T(locale, key string) string {
	if v, ok := table[locale][key]; ok {
		return v
	}
	if v, ok := table[English][key]; ok {
		return v
	}
	return key
}

// Has reports whether a locale is shipped with the tool.
func Has(locale string) bool {
	_, ok := table[locale]
	return ok
}

// MissingKeys returns the keys that a locale does not translate, so the
// package test can prove every locale is complete.
func MissingKeys(locale string) []string {
	local, ok := table[locale]
	if !ok {
		return nil
	}
	var missing []string
	for key := range table[English] {
		if _, ok := local[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// Join renders a list of locale codes.
func Join(locales []string) string {
	return strings.Join(locales, ",")
}

// Unknown returns the locale codes that are not shipped with the tool.
func Unknown(locales []string) []string {
	var out []string
	for _, l := range locales {
		if !Has(l) {
			out = append(out, l)
		}
	}
	return out
}

// Describe renders a short human description of a locale, used in headers.
func Describe(locale string) string {
	switch locale {
	case English:
		return "English"
	case Russian:
		return "Русский"
	case Spanish:
		return "Español"
	case Chinese:
		return "中文"
	}
	return fmt.Sprintf("%s (fallback to English)", locale)
}
