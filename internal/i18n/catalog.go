package i18n

import "fmt"

type Key string

const (
	ActivationTitle    Key = "activation.title"
	ActivationHeading  Key = "activation.heading"
	ActivationBody     Key = "activation.body"
	ActivationButton   Key = "activation.button"
	ActivationFootnote Key = "activation.footnote"
	ExpiredTitle       Key = "expired.title"
	ExpiredHeading     Key = "expired.heading"
	ExpiredBody        Key = "expired.body"
	ExpiredNote        Key = "expired.note"
	TrayOpen           Key = "tray.open"
	TrayReactivate     Key = "tray.reactivate"
	TrayExit           Key = "tray.exit"
	TrayStop           Key = "tray.stop"
	NotifyDecision     Key = "notify.decision"
	NotifyAcceptance   Key = "notify.acceptance"
	NotifyBlocker      Key = "notify.blocker"
	NotifyReturn       Key = "notify.return"
	NotifyFooter       Key = "notify.footer"
	LabelSummary       Key = "label.summary"
	LabelTask          Key = "label.task"
	LabelRequest       Key = "label.request"
	LabelProject       Key = "label.project"
)

var messages = map[Locale]map[Key]string{
	EN: {
		ActivationTitle:    "Roooooom · Get started",
		ActivationHeading:  "Local coordination is ready",
		ActivationBody:     "Confirm that you are the owner, then add a project. Each agent keeps working in its own host.",
		ActivationButton:   "Get started",
		ActivationFootnote: "This activates only on this computer. The link is valid for five minutes.",
		ExpiredTitle:       "Reactivate Roooooom",
		ExpiredHeading:     "This activation page has expired or was already used",
		ExpiredBody:        "Choose “Reactivate” from the Roooooom tray, then select “Get started”. Without a tray, use the product launcher's reactivation entry.",
		ExpiredNote:        "This does not delete any project or coordination record.",
		TrayOpen:           "Open Roooooom",
		TrayReactivate:     "Reactivate",
		TrayExit:           "Exit tray (service keeps running)",
		TrayStop:           "Stop service and exit",
		NotifyDecision:     "A decision is waiting for you",
		NotifyAcceptance:   "A result is ready for your acceptance",
		NotifyBlocker:      "A blocker needs your action",
		NotifyReturn:       "Please handle it in Roooooom on this computer; phones only receive reminders.",
		NotifyFooter:       "This may repeat after an uncertain network result; the current state on this computer is authoritative.",
		LabelSummary:       "Summary: ",
		LabelTask:          "Task: ",
		LabelRequest:       "Request: ",
		LabelProject:       "Project: ",
	},
	ZHCN: {
		ActivationTitle:    "Roooooom · 开始使用",
		ActivationHeading:  "本机协作已准备好",
		ActivationBody:     "确认你的管理身份，接着加入项目。各 agent 继续在原本的宿主工作。",
		ActivationButton:   "开始使用",
		ActivationFootnote: "只在这台电脑启用，这个入口五分钟内有效。",
		ExpiredTitle:       "重新启用 Roooooom",
		ExpiredHeading:     "启用页已过期或已使用",
		ExpiredBody:        "请从 Roooooom 托盘选择“重新启用”，再点“开始使用”。没有托盘时，可使用本产品启动器的重新启用入口。",
		ExpiredNote:        "这不会删除项目或协作记录。",
		TrayOpen:           "打开 Roooooom",
		TrayReactivate:     "重新启用",
		TrayExit:           "退出托盘（服务继续）",
		TrayStop:           "停止服务并退出",
		NotifyDecision:     "有待你拍板的事项",
		NotifyAcceptance:   "有指定由你验收的成果",
		NotifyBlocker:      "有需要你处理的阻塞",
		NotifyReturn:       "请回到这台电脑的 Roooooom 处理；手机只能收提醒。",
		NotifyFooter:       "可能因网络结果未知而重复；请以本机当前状态为准。",
		LabelSummary:       "摘要：",
		LabelTask:          "任务：",
		LabelRequest:       "需求：",
		LabelProject:       "项目：",
	},
	ZHTW: {
		ActivationTitle:    "Roooooom · 開始使用",
		ActivationHeading:  "本機協作已準備好",
		ActivationBody:     "確認你的管理身分，接著加入專案。各 agent 繼續在原本的宿主工作。",
		ActivationButton:   "開始使用",
		ActivationFootnote: "只在這台電腦啟用，這個入口五分鐘內有效。",
		ExpiredTitle:       "重新啟用 Roooooom",
		ExpiredHeading:     "啟用頁已過期或已使用",
		ExpiredBody:        "請從 Roooooom 托盤選擇「重新啟用」，再點「開始使用」。沒有托盤時，可使用本產品啟動器的重新啟用入口。",
		ExpiredNote:        "這不會刪除專案或協作紀錄。",
		TrayOpen:           "開啟 Roooooom",
		TrayReactivate:     "重新啟用",
		TrayExit:           "退出托盤（服務繼續）",
		TrayStop:           "停止服務並退出",
		NotifyDecision:     "有待你拍板的事項",
		NotifyAcceptance:   "有指定由你驗收的候選",
		NotifyBlocker:      "有需要你處理的阻塞",
		NotifyReturn:       "請回到這台電腦的 Roooooom 決策頁處理；手機只能收提醒。",
		NotifyFooter:       "可能因網路結果未知而重複；請以本機決策頁目前狀態為準。",
		LabelSummary:       "摘要：",
		LabelTask:          "任務：",
		LabelRequest:       "需求：",
		LabelProject:       "專案：",
	},
	JA: {
		ActivationTitle:    "Roooooom · はじめる",
		ActivationHeading:  "ローカル連携の準備ができました",
		ActivationBody:     "オーナーであることを確認してから、プロジェクトを追加します。各 agent はそれぞれのホストで作業を続けます。",
		ActivationButton:   "はじめる",
		ActivationFootnote: "このコンピューターでのみ有効化されます。このリンクの有効期限は 5 分です。",
		ExpiredTitle:       "Roooooom を再有効化",
		ExpiredHeading:     "この有効化ページは期限切れか、すでに使用されています",
		ExpiredBody:        "Roooooom のトレイから「再有効化」を選び、「はじめる」を押してください。トレイがない場合は、製品ランチャーの再有効化を使ってください。",
		ExpiredNote:        "プロジェクトや連携の記録は削除されません。",
		TrayOpen:           "Roooooom を開く",
		TrayReactivate:     "再有効化",
		TrayExit:           "トレイを終了（サービスは継続）",
		TrayStop:           "サービスを停止して終了",
		NotifyDecision:     "あなたの決定を待っている事項があります",
		NotifyAcceptance:   "あなたの受け入れ確認を待つ成果があります",
		NotifyBlocker:      "あなたの対応が必要なブロッカーがあります",
		NotifyReturn:       "このコンピューターの Roooooom で対応してください。スマートフォンには通知のみが届きます。",
		NotifyFooter:       "ネットワークの結果が不明な場合は重複することがあります。このコンピューターの現在の状態が正です。",
		LabelSummary:       "概要：",
		LabelTask:          "タスク：",
		LabelRequest:       "依頼：",
		LabelProject:       "プロジェクト：",
	},
}

// T returns the message for l, falling back to English and then to the key.
func T(l Locale, k Key, args ...any) string {
	text, ok := messages[l][k]
	if !ok {
		text, ok = messages[EN][k]
	}
	if !ok {
		return string(k)
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}
