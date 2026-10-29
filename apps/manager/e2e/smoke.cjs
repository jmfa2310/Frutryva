const { chromium, expect } = require("@playwright/test");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");
const results = path.join(__dirname, "..", "test-results");
fs.mkdirSync(results, { recursive: true });
const base = process.env.MANAGER_TEST_URL || "http://127.0.0.1:18081",
  mail = process.env.MAILPIT_TEST_URL || "http://127.0.0.1:18025";
const nonce = crypto.randomBytes(6).toString("hex"),
  password = crypto.randomBytes(24).toString("base64url");
const owner = "browser-" + nonce + "@example.invalid",
  coach = "coach-" + nonce + "@example.invalid";
const errors = [];
async function code(email) {
  for (let i = 0; i < 100; i++) {
    const list = await (
      await fetch(
        mail + "/api/v1/search?query=" + encodeURIComponent("to:" + email),
      )
    ).json();
    for (const m of list.messages || []) {
      const detail = await (
        await fetch(mail + "/api/v1/message/" + m.ID)
      ).json();
      const token = (detail.Text || "").match(
        /(?<![A-Za-z0-9_-])[A-Za-z0-9_-]{43}(?![A-Za-z0-9_-])/,
      );
      if (token) return token[0];
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw Error("Local email missing");
}
async function register(p, email, name) {
  await p.goto(base);
  await p.getByRole("button", { name: "Criar conta", exact: true }).click();
  await p.getByLabel("Nome", { exact: true }).fill(name);
  await p.getByLabel("Email", { exact: true }).fill(email);
  await p.getByLabel("Palavra-passe", { exact: true }).fill(password);
  await p
    .locator("form")
    .getByRole("button", { name: "Criar conta", exact: true })
    .click();
  await p.getByLabel("Código de confirmação", { exact: true }).waitFor();
  await p
    .getByLabel("Código de confirmação", { exact: true })
    .fill(await code(email));
  await p.getByRole("button", { name: "Confirmar email", exact: true }).click();
  await expect(
    p.getByLabel("Código de confirmação", { exact: true }),
  ).toHaveCount(0);
  await expect(
    p.getByText("Email confirmado", { exact: true }).last(),
  ).toBeVisible();
}
async function create(p, name) {
  await p.getByRole("button", { name: "Os meus clubes", exact: true }).click();
  await p.getByRole("button", { name: "Criar clube", exact: true }).click();
  const d = p.getByRole("dialog");
  await d.getByLabel("Nome do clube", { exact: true }).fill(name);
  await d.getByLabel("Nome curto (opcional)", { exact: true }).fill("FUT");
  await d.getByRole("button", { name: "Criar clube", exact: true }).click();
  await expect(d).toHaveCount(0);
  await expect(p.getByRole("heading", { name, exact: true })).toBeVisible();
}
(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || undefined,
    headless: true,
    args: ["--no-sandbox"],
  });
  try {
    const context = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      }),
      p = await context.newPage();
    p.on("pageerror", (e) => errors.push(e.message));
    await register(p, owner, "Gestor de teste");
    await create(p, "Clube navegador A");
    await p
      .getByRole("button", { name: "Identidade visual", exact: true })
      .click();
    await p.getByLabel("Principal", { exact: true }).fill("#134e4a");
    await p
      .getByLabel("Emblema", { exact: true })
      .setInputFiles({
        name: "emblema.png",
        mimeType: "image/png",
        buffer: Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=",
          "base64",
        ),
      });
    await p
      .getByRole("button", { name: "Guardar e publicar", exact: true })
      .click();
    await expect(
      p.getByText("Identidade publicada. A nova versão está disponível.", {
        exact: true,
      }),
    ).toBeVisible();
    await p.getByLabel("Principal", { exact: true }).fill("#4338ca");
    await p
      .getByRole("button", { name: "Guardar e publicar", exact: true })
      .click();
    await expect(p.getByText("Versão 2", { exact: true })).toBeVisible();
    await p.getByRole("button", { name: "Restaurar", exact: true }).click();
    await p
      .getByRole("dialog")
      .getByRole("button", { name: "Confirmar restauro", exact: true })
      .click();
    await expect(p.getByText("Versão 3", { exact: true })).toBeVisible();
    await p.evaluate(() => window.scrollTo(0, 0));
    await p.screenshot({
      path: path.join(results, "branding.png"),
      fullPage: true,
    });
    console.log(
      "PASS browser: registration, email confirmation, create club, image upload, publication and rollback",
    );
    await create(p, "Clube navegador B");
    await p
      .getByLabel("Clube ativo", { exact: true })
      .selectOption({ label: "Clube navegador A" });
    await p
      .getByRole("button", { name: "Pessoas e convites", exact: true })
      .click();
    await p.getByLabel("Email do destinatário", { exact: true }).fill(coach);
    await p
      .getByRole("button", { name: "Enviar convite", exact: true })
      .click();
    await expect(
      p.getByText(
        "Convite enviado por email. O destinatário deve aceitá-lo na sua própria conta.",
        { exact: true },
      ),
    ).toBeVisible();
    const invitation = await code(coach);
    const mobileContext = await browser.newContext({
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
      }),
      mobile = await mobileContext.newPage();
    mobile.on("pageerror", (e) => errors.push(e.message));
    await register(mobile, coach, "Treinador de teste");
    await mobile
      .getByLabel("Código do convite", { exact: true })
      .fill(invitation);
    await mobile
      .getByRole("button", { name: "Aceitar convite", exact: true })
      .last()
      .click();
    await expect(
      mobile.getByRole("heading", {
        name: "Clube navegador A",
        level: 2,
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      mobile.getByRole("heading", { name: "Clube navegador B", exact: true }),
    ).toHaveCount(0);
    await mobile
      .getByRole("button", { name: "Identidade visual", exact: true })
      .click();
    await expect(
      mobile.getByText("Só de leitura", { exact: true }),
    ).toBeVisible();
    await expect(
      mobile.getByRole("button", { name: "Guardar e publicar", exact: true }),
    ).toHaveCount(0);
    await mobile.evaluate(() => window.scrollTo(0, 0));
    await mobile.screenshot({
      path: path.join(results, "mobile.png"),
      fullPage: true,
    });
    const overflow = await mobile.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1,
    );
    if (overflow) throw Error("Mobile horizontal overflow");
    console.log(
      "PASS browser: second club, invitations, mobile acceptance and coach read-only permissions",
    );
    await p
      .getByRole("button", { name: "Terminar sessão", exact: true })
      .click();
    await expect(
      p.getByRole("button", { name: "Entrar no Manager", exact: true }),
    ).toBeVisible();
    await p.getByLabel("Email", { exact: true }).fill(owner);
    await p.getByLabel("Palavra-passe", { exact: true }).fill(password);
    await p
      .getByRole("button", { name: "Entrar no Manager", exact: true })
      .click();
    await expect(
      p.getByRole("heading", {
        name: "Clube navegador A",
        level: 2,
        exact: true,
      }),
    ).toBeVisible();
    if (errors.length)
      throw Error("Browser runtime errors: " + errors.join("; "));
    console.log("PASS browser: logout, login and no JavaScript runtime errors");
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e.message);
  process.exit(1);
});
