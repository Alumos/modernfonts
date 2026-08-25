export type FontStyle = "maru" | "round" | "hei" | "serif" | "kai" | "li" | "mono" | "creative"
export type FontSeries = "super" | "original" | "apple" | "japanese"
export type FontWeight = "three" | "four" | "five" | "six" | "multi" | "variable" | "other"

export const fontStyleOptions = [
  { value: "all", label: "全部字形" },
  { value: "maru", label: "丸系" },
  { value: "round", label: "圆体" },
  { value: "hei", label: "黑体" },
  { value: "serif", label: "宋体 / 明朝" },
  { value: "kai", label: "楷书" },
  { value: "li", label: "隶书" },
  { value: "mono", label: "等宽" },
  { value: "creative", label: "创意 / 手写" },
  { value: "other", label: "其他字形" },
] as const

export const fontSeriesOptions = [
  { value: "all", label: "全部系列" },
  { value: "super", label: "超级系列" },
  { value: "original", label: "原创造字" },
  { value: "apple", label: "Apple / 苹方" },
  { value: "japanese", label: "日系 / 日版" },
] as const

export const fontWeightOptions = [
  { value: "all", label: "全部字重" },
  { value: "three", label: "三字重" },
  { value: "four", label: "四字重" },
  { value: "five", label: "五字重" },
  { value: "six", label: "六字重" },
  { value: "multi", label: "多字重" },
  { value: "variable", label: "动态字重" },
  { value: "other", label: "其他字重" },
] as const

export type StyleFilter = typeof fontStyleOptions[number]["value"]
export type SeriesFilter = typeof fontSeriesOptions[number]["value"]
export type WeightFilter = typeof fontWeightOptions[number]["value"]

export type FontFilters = {
  style: StyleFilter
  series: SeriesFilter
  weight: WeightFilter
}

export type FontProfile = {
  styles: FontStyle[]
  series: FontSeries[]
  weight: FontWeight
  tags: string[]
}

const styleLabels: Record<FontStyle, string> = {
  maru: "丸系",
  round: "圆体",
  hei: "黑体",
  serif: "宋体 / 明朝",
  kai: "楷书",
  li: "隶书",
  mono: "等宽",
  creative: "创意 / 手写",
}

const seriesLabels: Record<FontSeries, string> = {
  super: "超级系列",
  original: "原创造字",
  apple: "Apple / 苹方",
  japanese: "日系 / 日版",
}

const weightLabels: Record<FontWeight, string> = {
  three: "三字重",
  four: "四字重",
  five: "五字重",
  six: "六字重",
  multi: "多字重",
  variable: "动态字重",
  other: "其他字重",
}

function classifyWeight(name: string): FontWeight {
  if (/动态|可变|variable/i.test(name)) return "variable"
  if (name.includes("多字重")) return "multi"
  if (/六字重|6字重/.test(name)) return "six"
  if (/五字重|5字重/.test(name)) return "five"
  if (/四字重|4字重/.test(name)) return "four"
  if (/三字重|3字重/.test(name)) return "three"

  const sequence = name.match(/(\d{2,3}(?:\/\d{2,3})+)/)?.[1]
  switch (sequence?.split("/").length) {
    case 3: return "three"
    case 4: return "four"
    case 5: return "five"
    case 6: return "six"
    default: return "other"
  }
}

export function getFontProfile(name: string): FontProfile {
  const styles: FontStyle[] = []
  const series: FontSeries[] = []
  const withoutYen = name.replace(/日圆/g, "")

  if (name.includes("丸")) styles.push("maru")
  if (withoutYen.includes("圆")) styles.push("round")
  if (name.includes("黑")) styles.push("hei")
  if (/宋|明朝|明体/.test(name)) styles.push("serif")
  if (name.includes("楷")) styles.push("kai")
  if (name.includes("隶")) styles.push("li")
  if (/等宽|mono/i.test(name)) styles.push("mono")
  if (/手写|手帐|书法|行书|教科书|插画/.test(name)) styles.push("creative")

  if (name.includes("超级")) series.push("super")
  if (name.includes("原创造字")) series.push("original")
  if (/苹方|iOS/i.test(name)) series.push("apple")
  if (/日版|日系|日本|JA/i.test(name)) series.push("japanese")

  const weight = classifyWeight(name)
  const tags = [
    ...series.map((value) => seriesLabels[value]),
    ...styles.map((value) => styleLabels[value]),
    ...(weight === "other" ? [] : [weightLabels[weight]]),
  ]
  if (tags.length === 0) tags.push("其他字形")

  return { styles, series, weight, tags }
}

export function matchesFontProfile(profile: FontProfile, filters: FontFilters): boolean {
  const styleMatches = filters.style === "all"
    || (filters.style === "other" ? profile.styles.length === 0 : profile.styles.includes(filters.style))
  const seriesMatches = filters.series === "all" || profile.series.includes(filters.series)
  const weightMatches = filters.weight === "all" || profile.weight === filters.weight
  return styleMatches && seriesMatches && weightMatches
}
