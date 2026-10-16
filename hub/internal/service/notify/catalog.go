package notify

type message struct {
	Subject  string
	Greeting string
	Intro    string
	Action   string
	Outro    string
}

// catalog holds every email in every supported locale. {instance} and {name}
// are replaced before rendering.
var catalog = map[string]map[Kind]message{
	"en": {
		KindVerifyEmail: {
			Subject:  "Confirm your email for {instance}",
			Greeting: "Hi {name},",
			Intro:    "Confirm this email address to finish setting up your account on {instance}.",
			Action:   "Confirm email",
			Outro:    "The link expires in 24 hours. If you did not create this account, you can ignore this email.",
		},
		KindPasswordReset: {
			Subject:  "Reset your {instance} password",
			Greeting: "Hi {name},",
			Intro:    "Someone asked to reset the password of your account on {instance}.",
			Action:   "Choose a new password",
			Outro:    "The link works once and expires in 30 minutes. If you did not ask for this, ignore this email and your password stays the same.",
		},
		KindChangeEmail: {
			Subject:  "Confirm your new email for {instance}",
			Greeting: "Hi {name},",
			Intro:    "Confirm this address to make it the email of your account on {instance}.",
			Action:   "Confirm new email",
			Outro:    "The link expires in 24 hours. Until then your account keeps its current email. If you did not ask for this, ignore this email.",
		},
		KindTest: {
			Subject:  "Test email from {instance}",
			Greeting: "Hello,",
			Intro:    "Email delivery for {instance} works. Verification and password reset emails will arrive like this one.",
		},
	},
	"pt-BR": {
		KindVerifyEmail: {
			Subject:  "Confirme seu email no {instance}",
			Greeting: "Olá, {name},",
			Intro:    "Confirme este endereço de email para concluir a criação da sua conta no {instance}.",
			Action:   "Confirmar email",
			Outro:    "O link expira em 24 horas. Se você não criou esta conta, pode ignorar este email.",
		},
		KindPasswordReset: {
			Subject:  "Redefina sua senha do {instance}",
			Greeting: "Olá, {name},",
			Intro:    "Alguém pediu para redefinir a senha da sua conta no {instance}.",
			Action:   "Escolher uma nova senha",
			Outro:    "O link funciona uma vez e expira em 30 minutos. Se você não pediu isso, ignore este email e sua senha continua a mesma.",
		},
		KindChangeEmail: {
			Subject:  "Confirme seu novo email no {instance}",
			Greeting: "Olá, {name},",
			Intro:    "Confirme este endereço para que ele passe a ser o email da sua conta no {instance}.",
			Action:   "Confirmar novo email",
			Outro:    "O link expira em 24 horas. Até lá, sua conta continua com o email atual. Se você não pediu isso, ignore este email.",
		},
		KindTest: {
			Subject:  "Email de teste do {instance}",
			Greeting: "Olá,",
			Intro:    "O envio de emails do {instance} funciona. Os emails de verificação e de redefinição de senha chegarão como este.",
		},
	},
}
