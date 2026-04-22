package router

// DefaultRoutingConfig returns the default routing configuration with all
// multilingual keyword lists (EN, ZH, JA, RU, DE, ES, PT, KO, AR),
// dimension weights, tier boundaries, and tier model configs.
func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{
		Version: "2.0",

		Classifier: ClassifierConfig{
			LLMModel:              "google/gemini-2.5-flash",
			LLMMaxTokens:         10,
			LLMTemperature:       0,
			PromptTruncationChars: 500,
			CacheTTLMs:           3_600_000, // 1 hour
		},

		Scoring: defaultScoringConfig(),

		// Auto (balanced) tier configs
		Tiers: map[Tier]TierConfig{
			TierSimple: {
				Primary: "google/gemini-2.5-flash",
				Fallback: []string{
					"google/gemini-3-flash-preview",
					"deepseek/deepseek-chat",
					"moonshot/kimi-k2.5",
					"google/gemini-3.1-flash-lite",
					"google/gemini-2.5-flash-lite",
					"openai/gpt-5.4-nano",
					"xai/grok-4-fast-non-reasoning",
					"free/gpt-oss-120b",
				},
			},
			TierMedium: {
				Primary: "moonshot/kimi-k2.5",
				Fallback: []string{
					"google/gemini-3-flash-preview",
					"deepseek/deepseek-chat",
					"google/gemini-2.5-flash",
					"google/gemini-3.1-flash-lite",
					"google/gemini-2.5-flash-lite",
					"xai/grok-4-1-fast-non-reasoning",
					"xai/grok-3-mini",
				},
			},
			TierComplex: {
				Primary: "google/gemini-3.1-pro",
				Fallback: []string{
					"google/gemini-3-pro-preview",
					"google/gemini-3-flash-preview",
					"xai/grok-4-0709",
					"google/gemini-2.5-pro",
					"anthropic/claude-sonnet-4.6",
					"deepseek/deepseek-chat",
					"google/gemini-2.5-flash",
					"openai/gpt-5.4",
				},
			},
			TierReasoning: {
				Primary: "xai/grok-4-1-fast-reasoning",
				Fallback: []string{
					"xai/grok-4-fast-reasoning",
					"deepseek/deepseek-reasoner",
					"openai/o4-mini",
					"openai/o3",
				},
			},
		},

		// Eco tier configs - absolute cheapest
		EcoTiers: map[Tier]TierConfig{
			TierSimple: {
				Primary: "free/gpt-oss-120b",
				Fallback: []string{
					"free/gpt-oss-20b",
					"google/gemini-3.1-flash-lite",
					"openai/gpt-5.4-nano",
					"google/gemini-2.5-flash-lite",
					"xai/grok-4-fast-non-reasoning",
				},
			},
			TierMedium: {
				Primary: "google/gemini-3.1-flash-lite",
				Fallback: []string{
					"openai/gpt-5.4-nano",
					"google/gemini-2.5-flash-lite",
					"xai/grok-4-fast-non-reasoning",
					"google/gemini-2.5-flash",
				},
			},
			TierComplex: {
				Primary: "google/gemini-3.1-flash-lite",
				Fallback: []string{
					"google/gemini-2.5-flash-lite",
					"xai/grok-4-0709",
					"google/gemini-2.5-flash",
					"deepseek/deepseek-chat",
				},
			},
			TierReasoning: {
				Primary: "xai/grok-4-1-fast-reasoning",
				Fallback: []string{
					"xai/grok-4-fast-reasoning",
					"deepseek/deepseek-reasoner",
				},
			},
		},

		// Premium tier configs - best quality
		PremiumTiers: map[Tier]TierConfig{
			TierSimple: {
				Primary: "moonshot/kimi-k2.6",
				Fallback: []string{
					"moonshot/kimi-k2.5",
					"google/gemini-2.5-flash",
					"anthropic/claude-haiku-4.5",
					"google/gemini-2.5-flash-lite",
					"deepseek/deepseek-chat",
				},
			},
			TierMedium: {
				Primary: "openai/gpt-5.3-codex",
				Fallback: []string{
					"moonshot/kimi-k2.6",
					"moonshot/kimi-k2.5",
					"google/gemini-2.5-flash",
					"google/gemini-2.5-pro",
					"xai/grok-4-0709",
					"anthropic/claude-sonnet-4.6",
				},
			},
			TierComplex: {
				Primary: "anthropic/claude-opus-4.7",
				Fallback: []string{
					"anthropic/claude-opus-4.6",
					"anthropic/claude-sonnet-4.6",
					"xai/grok-4-0709",
					"moonshot/kimi-k2.6",
					"moonshot/kimi-k2.5",
					"openai/gpt-5.4",
					"deepseek/deepseek-chat",
					"free/qwen3-coder-480b",
				},
			},
			TierReasoning: {
				Primary: "anthropic/claude-sonnet-4.6",
				Fallback: []string{
					"anthropic/claude-opus-4.7",
					"anthropic/claude-opus-4.6",
					"xai/grok-4-1-fast-reasoning",
					"openai/o4-mini",
					"openai/o3",
				},
			},
		},

		// Agentic tier configs - multi-step autonomous tasks
		AgenticTiers: map[Tier]TierConfig{
			TierSimple: {
				Primary: "openai/gpt-4o-mini",
				Fallback: []string{
					"moonshot/kimi-k2.5",
					"anthropic/claude-haiku-4.5",
					"xai/grok-4-1-fast-non-reasoning",
				},
			},
			TierMedium: {
				Primary: "moonshot/kimi-k2.5",
				Fallback: []string{
					"moonshot/kimi-k2.6",
					"xai/grok-4-1-fast-non-reasoning",
					"openai/gpt-4o-mini",
					"anthropic/claude-haiku-4.5",
					"deepseek/deepseek-chat",
				},
			},
			TierComplex: {
				Primary: "anthropic/claude-sonnet-4.6",
				Fallback: []string{
					"anthropic/claude-opus-4.7",
					"anthropic/claude-opus-4.6",
					"xai/grok-4-0709",
					"moonshot/kimi-k2.6",
					"moonshot/kimi-k2.5",
					"openai/gpt-5.4",
					"deepseek/deepseek-chat",
					"free/qwen3-coder-480b",
				},
			},
			TierReasoning: {
				Primary: "anthropic/claude-sonnet-4.6",
				Fallback: []string{
					"anthropic/claude-opus-4.7",
					"anthropic/claude-opus-4.6",
					"xai/grok-4-1-fast-reasoning",
					"deepseek/deepseek-reasoner",
				},
			},
		},

		Promotions: []Promotion{
			{
				Name:      "GLM-5.1 Launch Promo ($0.001 flat)",
				StartDate: "2026-04-01",
				EndDate:   "2026-04-15",
				TierOverrides: map[Tier]PartialTierConfig{
					TierSimple: {Primary: "zai/glm-5.1"},
				},
				Profiles: []string{"auto"},
			},
		},

		Overrides: OverridesConfig{
			MaxTokensForceComplex:   100_000,
			StructuredOutputMinTier: TierMedium,
			AmbiguousDefaultTier:    TierMedium,
			AgenticMode:             nil, // nil = auto-detect via tools + agenticScore
		},
	}
}

func defaultScoringConfig() ScoringConfig {
	cfg := ScoringConfig{
		// Multilingual keywords: EN + ZH + JA + RU + DE + ES + PT + KO + AR
		CodeKeywords: []string{
			// English
			"function", "class", "import", "def", "SELECT", "async", "await",
			"const", "let", "var", "return", "```",
			// Chinese
			"函数", "类", "导入", "定义", "查询", "异步", "等待", "常量", "变量", "返回",
			// Japanese
			"関数", "クラス", "インポート", "非同期", "定数", "変数",
			// Russian
			"функция", "класс", "импорт", "определ", "запрос", "асинхронный",
			"ожидать", "константа", "переменная", "вернуть",
			// German
			"funktion", "klasse", "importieren", "definieren", "abfrage",
			"asynchron", "erwarten", "konstante", "variable", "zurückgeben",
			// Spanish
			"función", "clase", "importar", "definir", "consulta",
			"asíncrono", "esperar", "constante", "variable", "retornar",
			// Portuguese
			"função", "classe", "importar", "definir", "consulta",
			"assíncrono", "aguardar", "constante", "variável", "retornar",
			// Korean
			"함수", "클래스", "가져오기", "정의", "쿼리", "비동기", "대기", "상수", "변수", "반환",
			// Arabic
			"دالة", "فئة", "استيراد", "تعريف", "استعلام", "غير متزامن",
			"انتظار", "ثابت", "متغير", "إرجاع",
		},
		ReasoningKeywords: []string{
			"prove", "theorem", "derive", "step by step", "chain of thought",
			"formally", "mathematical", "proof", "logically",
			"证明", "定理", "推导", "逐步", "思维链", "形式化", "数学", "逻辑",
			"証明", "定理", "導出", "ステップバイステップ", "論理的",
			"доказать", "докажи", "доказательств", "теорема", "вывести",
			"шаг за шагом", "пошагово", "поэтапно", "цепочка рассуждений",
			"рассуждени", "формально", "математически", "логически",
			"beweisen", "beweis", "theorem", "ableiten", "schritt für schritt",
			"gedankenkette", "formal", "mathematisch", "logisch",
			"demostrar", "teorema", "derivar", "paso a paso",
			"cadena de pensamiento", "formalmente", "matemático", "prueba", "lógicamente",
			"provar", "teorema", "derivar", "passo a passo",
			"cadeia de pensamento", "formalmente", "matemático", "prova", "logicamente",
			"증명", "정리", "도출", "단계별", "사고의 연쇄", "형식적", "수학적", "논리적",
			"إثبات", "نظرية", "اشتقاق", "خطوة بخطوة", "سلسلة التفكير",
			"رسمياً", "رياضي", "برهان", "منطقياً",
		},
		SimpleKeywords: []string{
			"what is", "define", "translate", "hello", "yes or no",
			"capital of", "how old", "who is", "when was",
			"什么是", "定义", "翻译", "你好", "是否", "首都", "多大", "谁是", "何时",
			"とは", "定義", "翻訳", "こんにちは", "はいかいいえ", "首都", "誰",
			"что такое", "определение", "перевести", "переведи", "привет",
			"да или нет", "столица", "сколько лет", "кто такой", "когда", "объясни",
			"was ist", "definiere", "übersetze", "hallo", "ja oder nein",
			"hauptstadt", "wie alt", "wer ist", "wann", "erkläre",
			"qué es", "definir", "traducir", "hola", "sí o no",
			"capital de", "cuántos años", "quién es", "cuándo",
			"o que é", "definir", "traduzir", "olá", "sim ou não",
			"capital de", "quantos anos", "quem é", "quando",
			"무엇", "정의", "번역", "안녕하세요", "예 또는 아니오", "수도", "누구", "언제",
			"ما هو", "تعريف", "ترجم", "مرحبا", "نعم أو لا", "عاصمة", "من هو", "متى",
		},
		TechnicalKeywords: []string{
			"algorithm", "optimize", "architecture", "distributed",
			"kubernetes", "microservice", "database", "infrastructure",
			"算法", "优化", "架构", "分布式", "微服务", "数据库", "基础设施",
			"アルゴリズム", "最適化", "アーキテクチャ", "分散", "マイクロサービス", "データベース",
			"алгоритм", "оптимизировать", "оптимизаци", "оптимизируй", "архитектура",
			"распределённый", "микросервис", "база данных", "инфраструктура",
			"algorithmus", "optimieren", "architektur", "verteilt", "kubernetes",
			"mikroservice", "datenbank", "infrastruktur",
			"algoritmo", "optimizar", "arquitectura", "distribuido",
			"microservicio", "base de datos", "infraestructura",
			"algoritmo", "otimizar", "arquitetura", "distribuído",
			"microsserviço", "banco de dados", "infraestrutura",
			"알고리즘", "최적화", "아키텍처", "분산", "마이크로서비스", "데이터베이스", "인프라",
			"خوارزمية", "تحسين", "بنية", "موزع", "خدمة مصغرة", "قاعدة بيانات", "بنية تحتية",
		},
		CreativeKeywords: []string{
			"story", "poem", "compose", "brainstorm", "creative", "imagine", "write a",
			"故事", "诗", "创作", "头脑风暴", "创意", "想象", "写一个",
			"物語", "詩", "作曲", "ブレインストーム", "創造的", "想像",
			"история", "рассказ", "стихотворение", "сочинить", "сочини",
			"мозговой штурм", "творческий", "представить", "придумай", "напиши",
			"geschichte", "gedicht", "komponieren", "brainstorming", "kreativ",
			"vorstellen", "schreibe", "erzählung",
			"historia", "poema", "componer", "lluvia de ideas", "creativo", "imaginar", "escribe",
			"história", "poema", "compor", "criativo", "imaginar", "escreva",
			"이야기", "시", "작곡", "브레인스토밍", "창의적", "상상", "작성",
			"قصة", "قصيدة", "تأليف", "عصف ذهني", "إبداعي", "تخيل", "اكتب",
		},
		ImperativeVerbs: []string{
			"build", "create", "implement", "design", "develop",
			"construct", "generate", "deploy", "configure", "set up",
			"构建", "创建", "实现", "设计", "开发", "生成", "部署", "配置", "设置",
			"構築", "作成", "実装", "設計", "開発", "生成", "デプロイ", "設定",
			"построить", "построй", "создать", "создай", "реализовать", "реализуй",
			"спроектировать", "разработать", "разработай", "сконструировать",
			"сгенерировать", "сгенерируй", "развернуть", "разверни", "настроить", "настрой",
			"erstellen", "bauen", "implementieren", "entwerfen", "entwickeln",
			"konstruieren", "generieren", "bereitstellen", "konfigurieren", "einrichten",
			"construir", "crear", "implementar", "diseñar", "desarrollar",
			"generar", "desplegar", "configurar",
			"construir", "criar", "implementar", "projetar", "desenvolver",
			"gerar", "implantar", "configurar",
			"구축", "생성", "구현", "설계", "개발", "배포", "설정",
			"بناء", "إنشاء", "تنفيذ", "تصميم", "تطوير", "توليد", "نشر", "إعداد",
		},
		ConstraintIndicators: []string{
			"under", "at most", "at least", "within", "no more than",
			"o(", "maximum", "minimum", "limit", "budget",
			"不超过", "至少", "最多", "在内", "最大", "最小", "限制", "预算",
			"以下", "最大", "最小", "制限", "予算",
			"не более", "не менее", "как минимум", "в пределах",
			"максимум", "минимум", "ограничение", "бюджет",
			"höchstens", "mindestens", "innerhalb", "nicht mehr als",
			"maximal", "minimal", "grenze", "budget",
			"como máximo", "al menos", "dentro de", "no más de",
			"máximo", "mínimo", "límite", "presupuesto",
			"no máximo", "pelo menos", "dentro de", "não mais que",
			"máximo", "mínimo", "limite", "orçamento",
			"이하", "이상", "최대", "최소", "제한", "예산",
			"على الأكثر", "على الأقل", "ضمن", "لا يزيد عن", "أقصى", "أدنى", "حد", "ميزانية",
		},
		OutputFormatKeywords: []string{
			"json", "yaml", "xml", "table", "csv", "markdown", "schema", "format as", "structured",
			"表格", "格式化为", "结构化",
			"テーブル", "フォーマット", "構造化",
			"таблица", "форматировать как", "структурированный",
			"tabelle", "formatieren als", "strukturiert",
			"tabla", "formatear como", "estructurado",
			"tabela", "formatar como", "estruturado",
			"테이블", "형식", "구조화",
			"جدول", "تنسيق", "منظم",
		},
		ReferenceKeywords: []string{
			"above", "below", "previous", "following", "the docs",
			"the api", "the code", "earlier", "attached",
			"上面", "下面", "之前", "接下来", "文档", "代码", "附件",
			"上記", "下記", "前の", "次の", "ドキュメント", "コード",
			"выше", "ниже", "предыдущий", "следующий", "документация", "код", "ранее", "вложение",
			"oben", "unten", "vorherige", "folgende", "dokumentation", "der code", "früher", "anhang",
			"arriba", "abajo", "anterior", "siguiente", "documentación", "el código", "adjunto",
			"acima", "abaixo", "anterior", "seguinte", "documentação", "o código", "anexo",
			"위", "아래", "이전", "다음", "문서", "코드", "첨부",
			"أعلاه", "أدناه", "السابق", "التالي", "الوثائق", "الكود", "مرفق",
		},
		NegationKeywords: []string{
			"don't", "do not", "avoid", "never", "without", "except", "exclude", "no longer",
			"不要", "避免", "从不", "没有", "除了", "排除",
			"しないで", "避ける", "決して", "なしで", "除く",
			"не делай", "не надо", "нельзя", "избегать", "никогда",
			"без", "кроме", "исключить", "больше не",
			"nicht", "vermeide", "niemals", "ohne", "außer", "ausschließen", "nicht mehr",
			"no hagas", "evitar", "nunca", "sin", "excepto", "excluir",
			"não faça", "evitar", "nunca", "sem", "exceto", "excluir",
			"하지 마", "피하다", "절대", "없이", "제외",
			"لا تفعل", "تجنب", "أبداً", "بدون", "باستثناء", "استبعاد",
		},
		DomainSpecificKeywords: []string{
			"quantum", "fpga", "vlsi", "risc-v", "asic", "photonics",
			"genomics", "proteomics", "topological", "homomorphic",
			"zero-knowledge", "lattice-based",
			"量子", "光子学", "基因组学", "蛋白质组学", "拓扑", "同态", "零知识", "格密码",
			"量子", "フォトニクス", "ゲノミクス", "トポロジカル",
			"квантовый", "фотоника", "геномика", "протеомика",
			"топологический", "гомоморфный", "с нулевым разглашением", "на основе решёток",
			"quanten", "photonik", "genomik", "proteomik", "topologisch",
			"homomorph", "zero-knowledge", "gitterbasiert",
			"cuántico", "fotónica", "genómica", "proteómica", "topológico", "homomórfico",
			"quântico", "fotônica", "genômica", "proteômica", "topológico", "homomórfico",
			"양자", "포토닉스", "유전체학", "위상", "동형",
			"كمي", "ضوئيات", "جينوميات", "طوبولوجي", "تماثلي",
		},
		AgenticTaskKeywords: []string{
			// English - File operations
			"read file", "read the file", "look at", "check the", "open the",
			"edit", "modify", "update the", "change the", "write to", "create file",
			// English - Execution
			"execute", "deploy", "install", "npm", "pip", "compile",
			// English - Multi-step
			"after that", "and also", "once done", "step 1", "step 2",
			// English - Iterative
			"fix", "debug", "until it works", "keep trying", "iterate",
			"make sure", "verify", "confirm",
			// Chinese
			"读取文件", "查看", "打开", "编辑", "修改", "更新", "创建",
			"执行", "部署", "安装", "第一步", "第二步", "修复", "调试", "直到", "确认", "验证",
			// Spanish
			"leer archivo", "editar", "modificar", "actualizar", "ejecutar",
			"desplegar", "instalar", "paso 1", "paso 2", "arreglar", "depurar", "verificar",
			// Portuguese
			"ler arquivo", "editar", "modificar", "atualizar", "executar",
			"implantar", "instalar", "passo 1", "passo 2", "corrigir", "depurar", "verificar",
			// Korean
			"파일 읽기", "편집", "수정", "업데이트", "실행", "배포", "설치",
			"단계 1", "단계 2", "디버그", "확인",
			// Arabic
			"قراءة ملف", "تحرير", "تعديل", "تحديث", "تنفيذ", "نشر", "تثبيت",
			"الخطوة 1", "الخطوة 2", "إصلاح", "تصحيح", "تحقق",
		},

		DimensionWeights: map[string]float64{
			"tokenCount":          0.08,
			"codePresence":        0.15,
			"reasoningMarkers":    0.18,
			"technicalTerms":      0.10,
			"creativeMarkers":     0.05,
			"simpleIndicators":    0.02,
			"multiStepPatterns":   0.12,
			"questionComplexity":  0.05,
			"imperativeVerbs":     0.03,
			"constraintCount":     0.04,
			"outputFormat":        0.03,
			"referenceComplexity": 0.02,
			"negationComplexity":  0.01,
			"domainSpecificity":   0.02,
			"agenticTask":         0.04,
		},

		ConfidenceSteepness: 12,
		ConfidenceThreshold: 0.7,
	}

	cfg.TokenCountThresholds.Simple = 50
	cfg.TokenCountThresholds.Complex = 500
	cfg.TierBoundaries.SimpleMedium = 0.0
	cfg.TierBoundaries.MediumComplex = 0.3
	cfg.TierBoundaries.ComplexReasoning = 0.5

	return cfg
}
