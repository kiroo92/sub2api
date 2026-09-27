<template>
  <div class="landing" data-testid="default-home">
    <header class="landing-header">
      <nav class="landing-shell header-inner" :aria-label="t('home.landing.navigation')">
        <router-link to="/home" class="brand" :aria-label="siteName">
          <img :src="siteLogo || '/logo.svg'" alt="" />
          <span>{{ siteName }}</span>
        </router-link>
        <div class="nav-actions">
          <a class="nav-link section-link" href="#features">{{ t('home.landing.features') }}</a>
          <a v-if="showModelPlaza" class="nav-link section-link" href="#models">{{
            t('home.landing.models')
          }}</a>
          <a
            v-if="docUrl"
            class="nav-link docs-link"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            >{{ t('home.docs') }}</a
          >
          <LocaleSwitcher />
          <button
            type="button"
            class="theme-button"
            :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="$emit('toggleTheme')"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
          <router-link :to="entryPath" class="nav-login">
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="landing-shell">
      <section class="hero" aria-labelledby="home-title">
        <div class="hero-eyebrow"><span></span>{{ t('home.landing.eyebrow') }}</div>
        <h1 id="home-title">{{ t('home.landing.title') }}</h1>
        <p class="hero-subtitle">{{ siteSubtitle }}</p>
        <p class="hero-description">{{ t('home.landing.description') }}</p>
        <div class="hero-actions">
          <router-link :to="entryPath" class="landing-button primary-button">
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
          <router-link
            v-if="showModelPlaza"
            to="/model-plaza"
            class="landing-button secondary-button"
          >
            <Icon name="grid" size="sm" />{{ t('home.landing.browseModels') }}
          </router-link>
          <a v-else href="#quickstart" class="landing-button secondary-button">
            <Icon name="terminal" size="sm" />{{ t('home.landing.quickstart') }}
          </a>
        </div>

        <div class="highlights">
          <div>
            <strong>1</strong><span>{{ t('home.landing.singleKey') }}</span>
          </div>
          <div>
            <strong>3</strong><span>{{ t('home.landing.apiFormats') }}</span>
          </div>
          <div>
            <strong>Token</strong><span>{{ t('home.tags.realtimeBilling') }}</span>
          </div>
          <div>
            <strong>SDK</strong><span>{{ t('home.landing.sdkReady') }}</span>
          </div>
        </div>
        <div class="provider-strip" :aria-label="t('home.landing.ecosystem')">
          <span v-for="provider in providers" :key="provider.model" class="provider-wordmark">
            <span class="provider-mark"><ModelIcon :model="provider.model" size="23px" /></span>
            {{ provider.name }}
          </span>
        </div>
      </section>

      <section id="features" class="features-section" :aria-label="t('home.landing.features')">
        <div class="feature-grid">
          <article class="feature-card">
            <div class="feature-art ecosystem-art" aria-hidden="true">
              <span
                v-for="(model, index) in illustrationModels"
                :key="index"
                class="floating-model"
              >
                <ModelIcon :model="model" size="22px" />
              </span>
            </div>
            <div class="feature-body">
              <h2>{{ t('home.features.unifiedGateway') }}</h2>
              <p>{{ t('home.features.unifiedGatewayDesc') }}</p>
              <router-link v-if="showModelPlaza" to="/model-plaza" class="text-link"
                >{{ t('home.landing.browseAll') }}<Icon name="arrowRight" size="xs"
              /></router-link>
              <a v-else href="#quickstart" class="text-link"
                >{{ t('home.landing.quickstart') }}<Icon name="arrowRight" size="xs"
              /></a>
            </div>
          </article>
          <article class="feature-card">
            <div class="feature-art routing-art" aria-hidden="true">
              <span class="routing-source">{{ siteName }} API</span>
              <svg class="routing-lines" viewBox="0 0 240 65" fill="none">
                <path d="M120 0v12C120 40 40 23 40 65M120 0v65M120 0v12c0 28 80 11 80 53" />
              </svg>
              <div class="routing-targets">
                <span v-for="model in ['claude', 'gpt', 'gemini']" :key="model"
                  ><ModelIcon :model="model" size="25px"
                /></span>
              </div>
            </div>
            <div class="feature-body">
              <h2>{{ t('home.features.multiAccount') }}</h2>
              <p>{{ t('home.features.multiAccountDesc') }}</p>
              <a href="#quickstart" class="text-link"
                >{{ t('home.landing.learnMore') }}<Icon name="arrowRight" size="xs"
              /></a>
            </div>
          </article>
          <article class="feature-card">
            <div class="feature-art usage-art" aria-hidden="true">
              <div class="usage-preview">
                <div>{{ t('home.landing.usagePreview') }}<Icon name="chart" size="sm" /></div>
                <i></i><i></i><i></i><i></i>
              </div>
            </div>
            <div class="feature-body">
              <h2>{{ t('home.features.balanceQuota') }}</h2>
              <p>{{ t('home.features.balanceQuotaDesc') }}</p>
              <router-link to="/key-usage" class="text-link"
                >{{ t('home.landing.viewUsage') }}<Icon name="arrowRight" size="xs"
              /></router-link>
            </div>
          </article>
          <article class="feature-card">
            <div class="feature-art security-art" aria-hidden="true">
              <div class="security-orbit">
                <div class="security-shield">
                  <Icon name="shield" size="xl" /><span><Icon name="check" size="sm" /></span>
                </div>
              </div>
            </div>
            <div class="feature-body">
              <h2>{{ t('home.landing.controlTitle') }}</h2>
              <p>{{ t('home.landing.controlDescription') }}</p>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="text-link"
                >{{ t('home.docs') }}<Icon name="externalLink" size="xs"
              /></a>
              <router-link v-else :to="entryPath" class="text-link"
                >{{ t('home.dashboard') }}<Icon name="arrowRight" size="xs"
              /></router-link>
            </div>
          </article>
        </div>
      </section>

      <section
        v-if="showModelPlaza"
        id="models"
        class="models-section"
        aria-labelledby="models-title"
        :aria-busy="modelsLoading"
      >
        <div class="section-heading">
          <div>
            <span class="section-eyebrow">{{ t('home.landing.modelsEyebrow') }}</span>
            <h2 id="models-title">{{ t('home.providers.title') }}</h2>
            <p v-if="modelFamilies.length">
              {{
                t('home.landing.modelSummary', {
                  models: modelCount,
                  platforms: modelFamilies.length,
                })
              }}
            </p>
            <p v-else>{{ t('home.providers.description') }}</p>
          </div>
          <router-link to="/model-plaza" class="text-link"
            >{{ t('home.landing.viewAll') }}<Icon name="arrowRight" size="sm"
          /></router-link>
        </div>
        <div v-if="modelsLoading" class="model-grid" role="status">
          <span class="sr-only">{{ t('common.loading') }}</span>
          <div v-for="n in 6" :key="n" class="model-skeleton"><i></i><i></i><i></i></div>
        </div>
        <div v-else-if="modelsFailed" class="catalog-notice" role="status">
          <Icon name="globe" size="lg" />
          <p>{{ t('home.landing.modelsUnavailable') }}</p>
          <button type="button" class="text-link" @click="retryCount++">
            {{ t('home.landing.retry') }}<Icon name="refresh" size="sm" />
          </button>
        </div>
        <div v-else-if="modelFamilies.length" class="model-grid">
          <router-link
            v-for="family in modelFamilies.slice(0, 6)"
            :key="family.platform"
            to="/model-plaza"
            class="model-card"
          >
            <div class="model-card-heading">
              <span class="model-avatar"><ModelIcon :model="family.example" size="27px" /></span>
              <div>
                <h3>{{ platformLabel(family.platform) }}</h3>
                <p>{{ t('home.landing.groupCount', { count: family.groups.size }) }}</p>
              </div>
              <Icon name="arrowRight" size="sm" class="model-arrow" />
            </div>
            <div class="model-card-details">
              <div>
                <span>{{ t('home.landing.availableModels') }}</span
                ><strong>{{ family.models.size }}</strong>
              </div>
              <span class="model-status"><span></span>{{ t('home.providers.supported') }}</span>
            </div>
          </router-link>
        </div>
        <div v-else class="catalog-notice">
          <Icon name="grid" size="lg" />
          <p>{{ t('home.landing.modelsEmpty') }}</p>
        </div>
      </section>

      <section id="quickstart" class="quickstart-section" aria-labelledby="quickstart-title">
        <div class="section-heading">
          <div>
            <span class="section-eyebrow">{{ t('home.landing.quickstartEyebrow') }}</span>
            <h2 id="quickstart-title">{{ t('home.landing.quickstartTitle') }}</h2>
            <p>{{ t('home.landing.quickstartDescription') }}</p>
          </div>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-link"
            >{{ t('home.viewDocs') }}<Icon name="externalLink" size="sm"
          /></a>
        </div>
        <div class="steps-grid">
          <article class="step">
            <div class="step-heading">
              <span>01</span>
              <h3>{{ t('home.landing.stepAccount') }}</h3>
            </div>
            <p>{{ t('home.landing.stepAccountDescription') }}</p>
            <div class="step-visual account-visual" aria-hidden="true">
              <Icon name="user" size="md" />
              <div><i></i><i></i></div>
              <span><Icon name="check" size="sm" /></span>
            </div>
          </article>
          <article class="step">
            <div class="step-heading">
              <span>02</span>
              <h3>{{ t('home.landing.stepModel') }}</h3>
            </div>
            <p>{{ t('home.landing.stepModelDescription') }}</p>
            <div class="step-visual models-visual" aria-hidden="true">
              <span
                ><ModelIcon model="claude" size="17px" />Claude<i></i
                ><Icon name="check" size="xs" /></span
              ><span
                ><ModelIcon model="gpt" size="17px" />GPT<i></i><Icon name="check" size="xs"
              /></span>
            </div>
          </article>
          <article class="step">
            <div class="step-heading">
              <span>03</span>
              <h3>{{ t('home.landing.stepKey') }}</h3>
            </div>
            <p>{{ t('home.landing.stepKeyDescription') }}</p>
            <div class="step-visual key-visual" aria-hidden="true">
              <Icon name="key" size="md" /><code>sk-••••••••••••••••••••</code
              ><Icon name="checkCircle" size="sm" />
            </div>
          </article>
        </div>
        <div class="protocols">
          <span>{{ t('home.landing.compatibleWith') }}</span
          ><code>OpenAI Chat Completions</code><code>OpenAI Responses</code
          ><code>Anthropic Messages</code>
        </div>
      </section>

      <section class="closing-cta" aria-labelledby="cta-title">
        <span class="cta-symbol" aria-hidden="true"><Icon name="terminal" size="lg" /></span>
        <h2 id="cta-title">{{ t('home.landing.ctaTitle') }}</h2>
        <p>{{ t('home.landing.ctaDescription') }}</p>
        <router-link :to="ctaPath" class="landing-button primary-button">
          {{
            isAuthenticated
              ? t('home.goToDashboard')
              : registrationEnabled
                ? t('home.landing.createAccount')
                : t('home.getStarted')
          }}<Icon name="arrowRight" size="sm" />
        </router-link>
      </section>
    </main>

    <footer class="landing-footer">
      <div class="landing-shell footer-inner">
        <div class="footer-brand">
          <router-link to="/home" class="brand"
            ><img :src="siteLogo || '/logo.svg'" alt="" /><span>{{ siteName }}</span></router-link
          >
          <p>
            &copy; {{ new Date().getFullYear() }} {{ siteName }}.
            {{ t('home.footer.allRightsReserved') }}
          </p>
        </div>
        <div class="footer-links">
          <h2>{{ t('home.landing.quickLinks') }}</h2>
          <router-link v-if="showModelPlaza" to="/model-plaza">{{
            t('nav.modelPlaza')
          }}</router-link
          ><router-link to="/key-usage">{{ t('keyUsage.title') }}</router-link
          ><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{
            t('home.docs')
          }}</a
          ><a v-else href="#quickstart">{{ t('home.landing.quickstart') }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { getModelPlaza, type ModelPlazaGroup } from '@/api/modelPlaza'
import { platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  siteName: string
  siteLogo: string
  siteSubtitle: string
  docUrl: string
  isAuthenticated: boolean
  dashboardPath: string
  isDark: boolean
  showModelPlaza: boolean
  registrationEnabled: boolean
}>()
defineEmits<{ toggleTheme: [] }>()
const { t } = useI18n()
const entryPath = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))
const ctaPath = computed(() =>
  props.isAuthenticated ? props.dashboardPath : props.registrationEnabled ? '/register' : '/login',
)

// Illustrative ecosystem marks; availability and counts below come from the catalog.
const providers = [
  { name: 'OpenAI', model: 'gpt' },
  { name: 'Claude', model: 'claude' },
  { name: 'Gemini', model: 'gemini' },
  { name: 'DeepSeek', model: 'deepseek' },
  { name: 'Qwen', model: 'qwen' },
  { name: 'Grok', model: 'grok' },
]
const illustrationModels = [
  'claude',
  'gpt',
  'deepseek',
  'gemini',
  'qwen',
  'kimi',
  'grok',
  'glm',
  'gemini',
  'deepseek',
  'claude',
  'gpt',
  'kimi',
  'qwen',
  'grok',
]
const groups = ref<ModelPlazaGroup[]>([])
const modelsLoading = ref(false)
const modelsFailed = ref(false)
const retryCount = ref(0)

watch(
  [() => props.showModelPlaza, () => props.isAuthenticated, retryCount],
  async ([enabled], _previous, onCleanup) => {
    groups.value = []
    modelsFailed.value = false
    modelsLoading.value = false
    if (!enabled) return
    const controller = new AbortController()
    onCleanup(() => controller.abort())
    modelsLoading.value = true
    try {
      const response = await getModelPlaza({ signal: controller.signal })
      if (!controller.signal.aborted) groups.value = response.groups ?? []
    } catch {
      if (!controller.signal.aborted) modelsFailed.value = true
    } finally {
      if (!controller.signal.aborted) modelsLoading.value = false
    }
  },
  { immediate: true },
)

const modelFamilies = computed(() => {
  const families = new Map<
    string,
    { platform: string; example: string; groups: Set<number>; models: Set<string> }
  >()
  for (const group of groups.value) {
    for (const model of group.models) {
      const platform = model.platform || group.platform
      let family = families.get(platform)
      if (!family) {
        family = { platform, example: model.name, groups: new Set(), models: new Set() }
        families.set(platform, family)
      }
      family.groups.add(group.id)
      family.models.add(model.name)
    }
  }
  return [...families.values()].sort(
    (a, b) => b.models.size - a.models.size || a.platform.localeCompare(b.platform),
  )
})
const modelCount = computed(() =>
  modelFamilies.value.reduce((total, family) => total + family.models.size, 0),
)
</script>

<style scoped>
.landing {
  --page: #fcfcfd;
  --surface: #fff;
  --art: #f8fafb;
  --ink: #111318;
  --muted: #697586;
  --line: #e6e9ee;
  --accent: #007faf;
  --accent-hover: #006b94;
  --tint: #eaf8fe;
  min-height: 100vh;
  background: var(--page);
  color: var(--ink);
  font-family:
    Inter,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    'Noto Sans SC',
    sans-serif;
  -webkit-font-smoothing: antialiased;
}
.dark .landing {
  --page: #101318;
  --surface: #161b22;
  --art: #13181f;
  --ink: #f1f5f9;
  --muted: #9aa8b9;
  --line: #29313c;
  --accent: #38bdf8;
  --accent-hover: #7dd3fc;
  --tint: #132c3d;
}
.landing-shell {
  width: min(1280px, calc(100% - 96px));
  margin-inline: auto;
}
.landing a,
.landing button {
  -webkit-tap-highlight-color: transparent;
}
.landing a:focus-visible,
.landing button:focus-visible {
  outline: 3px solid var(--accent);
  outline-offset: 5px;
}
.landing-header {
  border-bottom: 1px solid var(--line);
  background: var(--surface);
}
.header-inner {
  display: flex;
  min-height: 76px;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 11px;
  min-width: 0;
  font-size: 18px;
  font-weight: 650;
  letter-spacing: -0.5px;
}
.brand img {
  width: 31px;
  height: 31px;
  object-fit: contain;
  flex-shrink: 0;
}
.brand span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.nav-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 24px;
}
.nav-link {
  font-size: 14px;
  color: var(--muted);
  transition: color 0.2s;
}
.nav-link:hover,
.footer-links a:hover {
  color: var(--accent);
}
.theme-button {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  color: var(--muted);
}
.theme-button:hover {
  background: var(--art);
}
.nav-login {
  padding: 9px 22px;
  border-radius: 24px;
  background: var(--ink);
  color: var(--surface);
  font-size: 13px;
  font-weight: 600;
}
.hero {
  padding-top: 76px;
  text-align: center;
}
.hero-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
  letter-spacing: 0.1em;
  margin-bottom: 22px;
}
.hero-eyebrow span {
  width: 6px;
  height: 6px;
  background: var(--accent);
  border-radius: 50%;
  box-shadow: 0 0 0 4px var(--tint);
}
.hero h1 {
  margin: 0;
  font-size: clamp(38px, 4.6vw, 66px);
  font-weight: 750;
  line-height: 1.2;
  letter-spacing: -0.045em;
  text-wrap: balance;
}
.hero-subtitle {
  margin-top: 22px;
  font-size: 19px;
  color: var(--muted);
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.hero-description {
  margin: 8px auto 0;
  max-width: 630px;
  font-size: 14px;
  line-height: 1.8;
  color: var(--muted);
  text-wrap: balance;
}
.hero-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 31px;
}
.landing-button {
  display: inline-flex;
  min-height: 46px;
  min-width: 174px;
  align-items: center;
  justify-content: center;
  gap: 12px;
  border: 1px solid transparent;
  border-radius: 7px;
  padding: 12px 23px;
  font-size: 14px;
  font-weight: 600;
  transition:
    background 0.2s,
    border-color 0.2s,
    transform 0.2s;
}
.landing-button:hover {
  transform: translateY(-2px);
}
.primary-button {
  color: #fff;
  background: var(--accent);
}
.primary-button:hover {
  background: var(--accent-hover);
}
.dark .primary-button {
  color: #082435;
}
.secondary-button {
  background: var(--surface);
  border-color: var(--line);
}
.secondary-button:hover {
  border-color: var(--accent);
  background: var(--tint);
}
.highlights {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  max-width: 920px;
  margin: 62px auto 0;
}
.highlights > div {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.highlights > div + div::before {
  content: '';
  position: absolute;
  left: 0;
  top: 15px;
  width: 1px;
  height: 32px;
  background: var(--line);
}
.highlights strong {
  font-size: 36px;
  line-height: 1.2;
  font-weight: 650;
  letter-spacing: -1.5px;
}
.highlights span {
  font-size: 13px;
  color: var(--muted);
}
.provider-strip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 26px 38px;
  margin: 45px auto 0;
  padding: 22px 0;
}
.provider-wordmark {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--muted);
  font-size: 16px;
  font-weight: 550;
}
.provider-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 35px;
  height: 35px;
  border: 1px solid var(--line);
  border-radius: 50%;
  background: var(--surface);
}
.features-section {
  padding-top: 42px;
  scroll-margin-top: 28px;
}
.feature-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
}
.feature-card {
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface);
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}
.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 30px #13243c08;
}
.feature-art {
  position: relative;
  height: 175px;
  overflow: hidden;
  border-bottom: 1px solid var(--line);
  background: var(--art);
}
.feature-body {
  padding: 23px 21px 22px;
}
.feature-body h2 {
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.2px;
}
.feature-body p {
  min-height: 66px;
  margin-top: 10px;
  font-size: 13px;
  line-height: 1.85;
  color: var(--muted);
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--accent);
  font-size: 13px;
  line-height: 1.6;
  font-weight: 500;
}
.text-link:hover {
  color: var(--accent-hover);
}
.feature-body .text-link {
  margin-top: 18px;
}
.ecosystem-art {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  align-content: center;
  justify-items: center;
  gap: 16px 10px;
  padding: 15px 18px;
  background: linear-gradient(160deg, var(--surface), var(--art));
}
.ecosystem-art::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(0deg, var(--art), transparent 40%);
  pointer-events: none;
}
.floating-model {
  display: grid;
  place-items: center;
  width: 35px;
  height: 35px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 50%;
  box-shadow: 0 4px 8px #18223408;
}
.floating-model:nth-child(3n) {
  transform: translateY(7px);
}
.routing-art {
  display: flex;
  align-items: center;
  flex-direction: column;
  padding-top: 25px;
}
.routing-source {
  max-width: 85%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  padding: 7px 12px;
  border: 1px solid var(--line);
  border-radius: 5px;
  background: var(--surface);
}
.routing-lines {
  width: 80%;
  height: 60px;
  stroke: var(--line);
  stroke-width: 1.5;
}
.routing-targets {
  display: flex;
  justify-content: space-between;
  width: 70%;
  margin-top: -1px;
}
.routing-targets > span {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: 0 6px 12px #19253c0a;
}
.usage-art {
  display: grid;
  place-items: center;
}
.usage-preview {
  width: 75%;
  padding: 15px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
}
.usage-preview > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--muted);
  font-size: 11px;
  margin-bottom: 14px;
}
.usage-preview i {
  display: block;
  height: 7px;
  margin-top: 9px;
  width: 89%;
  border-radius: 4px;
  background: #7dd3fc;
}
.usage-preview i:nth-of-type(2) {
  width: 64%;
  background: #fcd76b;
}
.usage-preview i:nth-of-type(3) {
  width: 80%;
  background: #6ee7b7;
}
.usage-preview i:nth-of-type(4) {
  width: 48%;
  background: #c4b5fd;
}
.security-art {
  display: grid;
  place-items: center;
}
.security-orbit {
  display: grid;
  place-items: center;
  width: 135px;
  height: 135px;
  border: 1px dashed var(--line);
  border-radius: 50%;
}
.security-shield {
  display: grid;
  place-items: center;
  position: relative;
  width: 89px;
  height: 89px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 50%;
  color: var(--muted);
}
.security-shield > span {
  position: absolute;
  top: -1px;
  right: -2px;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  color: #10a577;
  background: #d5fae9;
}
.models-section,
.quickstart-section {
  padding-top: 82px;
  scroll-margin-top: 24px;
}
.section-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 20px;
  margin-bottom: 27px;
}
.section-eyebrow {
  display: block;
  color: var(--accent);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.1em;
  margin-bottom: 10px;
}
.section-heading h2 {
  font-size: 25px;
  line-height: 1.4;
  font-weight: 650;
  letter-spacing: -0.7px;
}
.section-heading p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
  margin-top: 7px;
}
.section-heading > .text-link {
  flex-shrink: 0;
  margin-bottom: 3px;
}
.model-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}
.model-card {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface);
  padding: 24px;
  transition:
    border-color 0.2s,
    transform 0.2s;
}
.model-card:hover {
  border-color: var(--accent);
  transform: translateY(-2px);
}
.model-card-heading {
  display: flex;
  align-items: center;
  gap: 15px;
}
.model-card-heading > div {
  min-width: 0;
}
.model-avatar {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  border: 1px solid var(--line);
  background: var(--art);
  flex-shrink: 0;
}
.model-card h3 {
  font-size: 17px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.model-card-heading p {
  margin-top: 3px;
  font-size: 12px;
  color: var(--muted);
}
.model-arrow {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--muted);
  opacity: 0;
  transition: opacity 0.2s;
}
.model-card:hover .model-arrow,
.model-card:focus-visible .model-arrow {
  opacity: 1;
}
.model-card-details {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
  margin-top: 21px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}
.model-card-details > div {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.model-card-details > div > span {
  color: var(--muted);
  font-size: 11px;
}
.model-card-details strong {
  font-size: 20px;
  line-height: 1;
  font-weight: 600;
}
.model-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--accent);
}
.model-status > span {
  width: 5px;
  height: 5px;
  background: currentColor;
  border-radius: 50%;
}
.catalog-notice {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 15px;
  min-height: 150px;
  padding: 25px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--muted);
  font-size: 14px;
  background: var(--surface);
}
.model-skeleton {
  height: 185px;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 25px;
  background: var(--surface);
}
.model-skeleton i {
  display: block;
  height: 16px;
  width: 60%;
  margin: 12px 0;
  background: var(--line);
  border-radius: 4px;
  animation: pulse 1.5s ease-in-out infinite;
}
.model-skeleton i:first-child {
  width: 40px;
  height: 40px;
  border-radius: 50%;
}
.model-skeleton i:last-child {
  width: 35%;
}
.steps-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 44px;
  margin-top: 36px;
}
.step-heading {
  display: flex;
  align-items: center;
  gap: 12px;
}
.step-heading > span {
  display: grid;
  place-items: center;
  width: 29px;
  height: 29px;
  border-radius: 50%;
  background: var(--tint);
  color: var(--accent);
  font-size: 11px;
  font-weight: 600;
}
.step-heading h3 {
  font-size: 16px;
  font-weight: 600;
}
.step > p {
  min-height: 52px;
  margin-top: 16px;
  font-size: 13px;
  line-height: 1.9;
  color: var(--muted);
}
.step-visual {
  margin-top: 25px;
  max-width: 290px;
  color: var(--accent);
}
.account-visual {
  display: flex;
  align-items: center;
  gap: 14px;
  height: 63px;
  padding: 12px 16px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: var(--surface);
}
.account-visual > div {
  flex: 1;
}
.account-visual i {
  display: block;
  width: 45%;
  height: 6px;
  background: var(--tint);
  border-radius: 4px;
}
.account-visual i + i {
  width: 80%;
  margin-top: 9px;
}
.account-visual > span {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--tint);
}
.models-visual {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.models-visual > span {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  font-size: 10px;
  border: 1px solid var(--line);
  border-radius: 5px;
  background: var(--surface);
}
.models-visual i {
  height: 5px;
  flex: 1;
  border-radius: 4px;
  background: var(--tint);
  margin-inline: 9px;
}
.key-visual {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 63px;
}
.key-visual code {
  padding: 12px 10px;
  border: 1px solid var(--line);
  border-radius: 5px;
  background: var(--surface);
  color: var(--muted);
  font-size: 10px;
  white-space: nowrap;
}
.protocols {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px 22px;
  margin-top: 36px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: 11px;
}
.protocols code {
  font-size: 11px;
}
.closing-cta {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 85px 0 90px;
}
.cta-symbol {
  display: grid;
  place-items: center;
  width: 43px;
  height: 43px;
  margin-bottom: 21px;
  border: 1px solid var(--line);
  border-radius: 11px;
  color: var(--accent);
  background: var(--surface);
}
.closing-cta h2 {
  font-size: 33px;
  font-weight: 650;
  letter-spacing: -1px;
  line-height: 1.4;
  text-wrap: balance;
}
.closing-cta p {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.8;
  margin-top: 14px;
}
.closing-cta .landing-button {
  margin-top: 27px;
}
.landing-footer {
  border-top: 1px solid var(--line);
  background: var(--surface);
}
.footer-inner {
  display: flex;
  justify-content: space-between;
  gap: 30px;
  padding-block: 44px;
}
.footer-brand {
  min-width: 0;
}
.footer-brand .brand {
  max-width: 100%;
  font-size: 16px;
}
.footer-brand p {
  margin-top: 17px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}
.footer-links {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  flex-shrink: 0;
  min-width: 140px;
  font-size: 12px;
  color: var(--muted);
}
.footer-links h2 {
  margin-bottom: 3px;
  color: var(--ink);
  font-weight: 600;
}
.dark .landing :deep(.model-icon path[fill='#000000']),
.dark .landing :deep(.model-icon path[fill='#16191E']) {
  fill: #e2e8f0;
}
@keyframes pulse {
  50% {
    opacity: 0.45;
  }
}
@media (min-width: 1500px) {
  .hero {
    padding-top: 90px;
  }
}
@media (max-width: 1100px) {
  .landing-shell {
    width: calc(100% - 64px);
  }
  .nav-actions {
    gap: 16px;
  }
  .feature-grid {
    gap: 14px;
  }
  .feature-body {
    padding: 20px 16px;
  }
  .feature-body p {
    min-height: 96px;
  }
  .provider-strip {
    gap: 20px;
  }
  .steps-grid {
    gap: 26px;
  }
}
@media (max-width: 900px) {
  .feature-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 20px;
  }
  .feature-body p {
    min-height: 48px;
  }
  .model-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .nav-actions {
    gap: 10px;
  }
  .section-link {
    display: none;
  }
  .brand {
    max-width: 240px;
  }
  .steps-grid {
    gap: 22px;
  }
  .step > p {
    min-height: 80px;
  }
  .key-visual {
    gap: 6px;
    flex-wrap: wrap;
  }
  .provider-wordmark {
    font-size: 14px;
  }
}
@media (max-width: 600px) {
  .landing-shell {
    width: calc(100% - 40px);
  }
  .header-inner {
    min-height: 68px;
    gap: 10px;
  }
  .brand {
    gap: 8px;
    font-size: 16px;
  }
  .brand img {
    width: 27px;
    height: 27px;
  }
  .nav-actions {
    gap: 4px;
  }
  .nav-login {
    padding: 8px 14px;
    font-size: 12px;
  }
  .docs-link {
    display: none;
  }
  .theme-button {
    width: 30px;
    height: 32px;
  }
  .hero {
    padding-top: 48px;
  }
  .hero-eyebrow {
    font-size: 10px;
    margin-bottom: 20px;
  }
  .hero h1 {
    font-size: clamp(32px, 8.6vw, 48px);
    letter-spacing: -0.05em;
  }
  .hero-subtitle {
    font-size: 16px;
    margin-top: 18px;
  }
  .hero-description {
    font-size: 12px;
    max-width: 300px;
    margin-top: 9px;
  }
  .hero-actions {
    margin-top: 25px;
    gap: 10px;
  }
  .landing-button {
    min-width: 0;
    padding: 11px 17px;
    font-size: 12px;
    min-height: 44px;
    gap: 8px;
  }
  .highlights {
    margin-top: 38px;
    gap: 25px 0;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .highlights strong {
    font-size: 30px;
  }
  .highlights span {
    font-size: 11px;
  }
  .highlights > div:nth-child(3)::before {
    display: none;
  }
  .provider-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 18px 10px;
    margin-top: 28px;
    padding-block: 18px;
    text-align: left;
  }
  .provider-wordmark {
    font-size: 12px;
    gap: 6px;
  }
  .provider-mark {
    width: 28px;
    height: 28px;
    border: 0;
    background: transparent;
  }
  .features-section {
    padding-top: 26px;
  }
  .feature-grid {
    gap: 14px;
  }
  .feature-art {
    height: 137px;
  }
  .feature-body {
    padding: 17px 14px;
  }
  .feature-body h2 {
    font-size: 14px;
  }
  .feature-body p {
    min-height: 112px;
    font-size: 12px;
    line-height: 1.8;
  }
  .feature-body .text-link {
    font-size: 12px;
    margin-top: 12px;
  }
  .ecosystem-art {
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
    padding: 12px;
  }
  .floating-model {
    width: 31px;
    height: 31px;
  }
  .routing-art {
    padding-top: 19px;
  }
  .routing-source {
    font-size: 9px;
    padding: 5px 8px;
  }
  .routing-lines {
    height: 42px;
  }
  .routing-targets {
    width: 80%;
  }
  .routing-targets > span {
    width: 29px;
    height: 29px;
  }
  .usage-preview {
    width: 86%;
    padding: 12px;
  }
  .usage-preview > div {
    font-size: 9px;
    margin-bottom: 10px;
  }
  .usage-preview i {
    height: 5px;
    margin-top: 8px;
  }
  .security-orbit {
    width: 112px;
    height: 112px;
  }
  .security-shield {
    width: 75px;
    height: 75px;
  }
  .models-section,
  .quickstart-section {
    padding-top: 49px;
  }
  .section-heading {
    gap: 12px;
    margin-bottom: 20px;
  }
  .section-heading h2 {
    font-size: 21px;
  }
  .section-heading p {
    font-size: 12px;
  }
  .section-heading > .text-link {
    font-size: 11px;
  }
  .model-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .model-card {
    padding: 20px;
  }
  .model-arrow {
    opacity: 1;
  }
  .steps-grid {
    grid-template-columns: 1fr;
    gap: 29px;
    margin-top: 27px;
  }
  .step {
    border-bottom: 1px solid var(--line);
    padding-bottom: 27px;
  }
  .step:last-child {
    border-bottom: 0;
    padding-bottom: 0;
  }
  .step > p {
    min-height: 0;
    margin-top: 11px;
  }
  .step-visual {
    margin-top: 15px;
    max-width: 100%;
  }
  .protocols {
    gap: 12px;
    margin-top: 24px;
  }
  .protocols > span {
    width: 100%;
  }
  .protocols code {
    font-size: 10px;
  }
  .closing-cta {
    padding: 54px 5px 56px;
  }
  .closing-cta h2 {
    font-size: 27px;
  }
  .closing-cta p {
    font-size: 12px;
  }
  .closing-cta .landing-button {
    min-width: 165px;
  }
  .footer-inner {
    flex-direction: column;
    padding-block: 30px;
    gap: 27px;
  }
  .footer-links {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 16px;
  }
  .footer-links h2 {
    width: 100%;
    margin-bottom: 0;
  }
}
@media (max-width: 359px) {
  .landing-shell {
    width: calc(100% - 28px);
  }
  .feature-grid {
    grid-template-columns: 1fr;
  }
  .feature-body p {
    min-height: 0;
  }
  .ecosystem-art {
    grid-template-columns: repeat(5, 1fr);
  }
  .hero-actions {
    flex-direction: column;
  }
}
@media (prefers-reduced-motion: reduce) {
  .landing *,
  .landing *::before,
  .landing *::after {
    animation: none !important;
    transition: none !important;
  }
}
</style>
