const { chromium, expect } = require("@playwright/test");
const { execFileSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..", "..", "..");
const results = path.join(__dirname, "..", "test-results");
const base = process.env.MANAGER_TEST_URL || "http://127.0.0.1:18081";
const mail = process.env.MAILPIT_TEST_URL || "http://127.0.0.1:18025";
const nonce = crypto.randomBytes(6).toString("hex");
const clubAName = "Clube navegador A " + nonce;
const clubBName = "Clube navegador B " + nonce;
const password = crypto.randomBytes(24).toString("base64url");
const emails = ["owner", "manager-a", "manager-b", "coach"].map(
  (role) => `browser-${role}-${nonce}@example.invalid`,
);
const sensitive = [password, ...emails];
const seenMessages = new Set();
const errors = [];

function bootstrapOwner(email) {
  const useDocker = process.env.E2E_BOOTSTRAP_OWNER === "1";
  const binary = process.env.E2E_OWNER_BINARY;
  if (useDocker === Boolean(binary)) {
    throw Error("Choose exactly one explicit owner bootstrap: E2E_BOOTSTRAP_OWNER=1 or E2E_OWNER_BINARY=/trusted/absolute/binary");
  }
  if (binary && (!path.isAbsolute(binary) || !fs.existsSync(binary) || !fs.statSync(binary).isFile())) {
    throw Error("E2E_OWNER_BINARY must be an existing trusted absolute executable path");
  }
  try {
    execFileSync(
      binary || "docker",
      binary
        ? ["owner", "set", email]
        : ["compose", "exec", "-T", "api", "/futryva", "owner", "set", email],
      { cwd: repoRoot, stdio: "pipe", timeout: 30000 },
    );
  } catch {
    throw Error("Explicit local owner bootstrap failed; check CLI/database configuration");
  }
}

async function code(email) {
  for (let attempt = 0; attempt < 150; attempt++) {
    const response = await fetch(mail + "/api/v1/search?query=" + encodeURIComponent("to:" + email));
    if (!response.ok) throw Error("Development inbox API unavailable");
    const list = await response.json();
    for (const message of list.messages || []) {
      if (seenMessages.has(message.ID)) continue;
      const response = await fetch(mail + "/api/v1/message/" + message.ID);
      if (!response.ok) throw Error("Development email unavailable");
      const detail = await response.json();
      seenMessages.add(message.ID);
      const token = (detail.Text || "").match(/(?<![A-Za-z0-9_-])[A-Za-z0-9_-]{43}(?![A-Za-z0-9_-])/);
      if (token) {
        sensitive.push(token[0]);
        return token[0];
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw Error("Expected development email did not arrive");
}

async function register(page, email, name) {
  await page.goto(base);
  await page.getByRole("button", { name: "Criar conta", exact: true }).click();
  await page.getByLabel("Nome", { exact: true }).fill(name);
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Palavra-passe", { exact: true }).fill(password);
  await page.locator("form").getByRole("button", { name: "Criar conta", exact: true }).click();
  await page.getByLabel("Código de confirmação", { exact: true }).waitFor();
  await page.getByLabel("Código de confirmação", { exact: true }).fill(await code(email));
  await page.getByRole("button", { name: "Confirmar email", exact: true }).click();
  await expect(page.getByLabel("Código de confirmação", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Email confirmado", { exact: true }).last()).toBeVisible();
}

async function provision(page, name, managerEmail) {
  await page.getByRole("button", { name: "Administração", exact: true }).click();
  const form = page.getByRole("form", { name: "Criar e atribuir clube", exact: true });
  await form.getByLabel("Nome do novo clube", { exact: true }).fill(name);
  await form.getByLabel("Nome curto do novo clube (opcional)", { exact: true }).fill("FUT");
  await form.getByLabel("Email do responsável", { exact: true }).fill(managerEmail);
  await form.getByRole("button", { name: "Criar e atribuir clube", exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByText("Clube criado e atribuído ao responsável indicado.", { exact: true })).toBeVisible();
}

async function publishedMenu(page, primary, secondary) {
  const rgb = (hex) => `rgb(${[1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16)).join(", ")})`;
  await expect(page.locator(".sidebar")).toHaveCSS("background-color", rgb(secondary));
  await expect(page.locator(".sidebar .nav-button.active")).toHaveCSS("background-color", rgb(primary));
}

(async () => {
  // Fail before creating test accounts unless local operator bootstrap was explicit.
  if ((process.env.E2E_BOOTSTRAP_OWNER === "1") === Boolean(process.env.E2E_OWNER_BINARY)) {
    throw Error("Choose E2E_BOOTSTRAP_OWNER=1 or E2E_OWNER_BINARY=/trusted/absolute/binary before running this development test");
  }
  fs.mkdirSync(results, { recursive: true });
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || undefined,
    headless: true,
    args: ["--no-sandbox"],
  });
  try {
    const desktop = { viewport: { width: 1440, height: 1000 } };
    const ownerContext = await browser.newContext(desktop);
    const managerContext = await browser.newContext(desktop);
    const secondContext = await browser.newContext(desktop);
    const ownerPage = await ownerContext.newPage();
    const manager = await managerContext.newPage();
    const secondManager = await secondContext.newPage();
    for (const page of [ownerPage, manager, secondManager]) {
      page.setDefaultTimeout(15000);
      page.on("pageerror", (error) => errors.push(error.message));
    }
    await register(ownerPage, emails[0], "Operador de teste");
    await register(manager, emails[1], "Gestor de teste A");
    await register(secondManager, emails[2], "Gestor de teste B");
    await manager.getByRole("button", { name: "O meu clube", exact: true }).click();
    await expect(manager.getByRole("heading", { name: "O seu clube ainda não foi atribuído.", exact: true })).toBeVisible();
    await expect(manager.getByRole("button", { name: "Criar clube", exact: true })).toHaveCount(0);
    await expect(manager.getByRole("button", { name: "Administração", exact: true })).toHaveCount(0);
    bootstrapOwner(emails[0]);
    await ownerPage.reload();
    await provision(ownerPage, clubAName, emails[1]);
    await provision(ownerPage, clubBName, emails[2]);
    await ownerPage.screenshot({ path: path.join(results, "owner-admin.png"), fullPage: true });
    console.log("PASS browser: verified accounts, explicit owner bootstrap and owner-only club provisioning");

    await manager.reload();
    await expect(manager.getByRole("heading", { name: clubAName, level: 2, exact: true })).toBeVisible();
    await expect(manager.getByRole("heading", { name: clubBName, exact: true })).toHaveCount(0);
    await expect(manager.getByLabel("Clube ativo", { exact: true })).toHaveCount(0);
    await expect(manager.getByRole("button", { name: "Criar clube", exact: true })).toHaveCount(0);
    await manager.getByRole("button", { name: "Identidade visual", exact: true }).click();
    const originalSidebar = await manager.locator(".sidebar").evaluate((element) => getComputedStyle(element).backgroundColor);
    await manager.getByRole("button", { name: "Ver paleta de Principal", exact: true }).click();
    const palette = manager.getByRole("group", { name: "Paleta de Principal", exact: true });
    const swatches = palette.getByRole("button");
    if ((await swatches.count()) < 40) throw Error("Expanded palette has fewer than forty selectable colors");
    const firstColor = (await swatches.first().getAttribute("aria-label")).match(/#[0-9A-Fa-f]{6}$/)[0];
    await swatches.first().click();
    await expect(manager.getByLabel("Principal", { exact: true })).toHaveValue(firstColor);
    await swatches.first().focus();
    await manager.keyboard.press("ArrowRight");
    await expect(swatches.nth(1)).toBeFocused();
    await manager.getByRole("button", { name: "Fechar paleta de Principal", exact: true }).click();
    if ((await manager.getByRole("button", { name: /^Aplicar / }).count()) < 12) throw Error("Branding presets are missing");
    await manager.getByLabel("Principal", { exact: true }).fill("#134e4a");
    await manager.getByLabel("Secundária", { exact: true }).fill("#052e2b");
    await manager.getByLabel("Emblema", { exact: true }).setInputFiles({
      name: "emblema.png",
      mimeType: "image/png",
      buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=", "base64"),
    });
    await expect(manager.locator(".sidebar")).toHaveCSS("background-color", originalSidebar);
    await manager.getByRole("button", { name: "Guardar e publicar", exact: true }).click();
    await expect(manager.getByText("Identidade publicada. A nova versão está disponível.", { exact: true })).toBeVisible();
    await publishedMenu(manager, "#134e4a", "#052e2b");
    await manager.getByLabel("Principal", { exact: true }).fill("#4338ca");
    await manager.getByRole("button", { name: "Guardar rascunho", exact: true }).click();
    await expect(manager.getByText("Rascunho guardado. A versão pública mantém-se até publicar.", { exact: true })).toBeVisible();
    await publishedMenu(manager, "#134e4a", "#052e2b");
    await manager.getByRole("button", { name: "Guardar e publicar", exact: true }).click();
    await expect(manager.getByText("Versão 2", { exact: true })).toBeVisible();
    await publishedMenu(manager, "#4338ca", "#052e2b");
    await manager.getByRole("button", { name: "Restaurar", exact: true }).click();
    await manager.getByRole("dialog").getByRole("button", { name: "Confirmar restauro", exact: true }).click();
    await expect(manager.getByText("Versão 3", { exact: true })).toBeVisible();
    await publishedMenu(manager, "#134e4a", "#052e2b");
    await manager.evaluate(() => window.scrollTo(0, 0));
    await manager.screenshot({ path: path.join(results, "branding.png"), fullPage: true });
    console.log("PASS browser: assigned club, expanded keyboard palette, image upload and published-only menu colors including rollback");

    await manager.getByRole("button", { name: "Pessoas e convites", exact: true }).click();
    await manager.getByLabel("Email do destinatário", { exact: true }).fill(emails[3]);
    await manager.getByRole("button", { name: "Enviar convite", exact: true }).click();
    await expect(manager.getByText("Convite enviado por email. O destinatário deve aceitá-lo na sua própria conta.", { exact: true })).toBeVisible();
    const invitation = await code(emails[3]);
    const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    const mobile = await mobileContext.newPage();
    mobile.setDefaultTimeout(15000);
    mobile.on("pageerror", (error) => errors.push(error.message));
    await register(mobile, emails[3], "Treinador de teste");
    await mobile.getByLabel("Código do convite", { exact: true }).fill(invitation);
    await mobile.getByRole("button", { name: "Aceitar convite", exact: true }).click();
    await expect(mobile.getByRole("heading", { name: clubAName, level: 2, exact: true })).toBeVisible();
    await expect(mobile.getByRole("heading", { name: clubBName, exact: true })).toHaveCount(0);
    await expect(mobile.getByLabel("Clube ativo", { exact: true })).toHaveCount(0);
    await mobile.getByRole("button", { name: "Identidade visual", exact: true }).click();
    await expect(mobile.getByText("Só de leitura", { exact: true })).toBeVisible();
    await expect(mobile.getByRole("button", { name: "Guardar e publicar", exact: true })).toHaveCount(0);
    await publishedMenu(mobile, "#134e4a", "#052e2b");
    await mobile.evaluate(() => window.scrollTo(0, 0));
    await mobile.screenshot({ path: path.join(results, "mobile.png"), fullPage: true });
    if (await mobile.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)) throw Error("Mobile horizontal overflow");
    console.log("PASS browser: mobile invitation acceptance, single-club access and coach read-only permissions");

    await manager.getByRole("button", { name: "Terminar sessão", exact: true }).click();
    await expect(manager.getByRole("button", { name: "Entrar no Manager", exact: true })).toBeVisible();
    await manager.getByLabel("Email", { exact: true }).fill(emails[1]);
    await manager.getByLabel("Palavra-passe", { exact: true }).fill(password);
    await manager.getByRole("button", { name: "Entrar no Manager", exact: true }).click();
    await expect(manager.getByRole("heading", { name: clubAName, level: 2, exact: true })).toBeVisible();
    await publishedMenu(manager, "#134e4a", "#052e2b");
    if (errors.length) throw Error("Browser JavaScript runtime error detected");
    console.log("PASS browser: logout/login retains assigned club and published menu without runtime errors");
  } finally {
    await browser.close();
  }
})().catch((error) => {
  let message = error.message;
  for (const value of sensitive) message = message.split(value).join("[redacted]");
  console.error(message);
  process.exit(1);
});
