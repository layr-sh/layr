<script setup lang="ts">
definePageMeta({
  layout: false,
});

const auth = useAuth();

const email = ref("");
const password = ref("");
const showPassword = ref(false);
const mfaPin = ref<number[]>([]);
const mfaCode = computed(() => mfaPin.value.join(""));
const isMFA = ref(false);
const errorMessage = ref("");
const isLoading = ref(false);

const slides = [
  "Eliminate the compromise between developer velocity and data sovereignty",
  "Open-source developer infrastructure built on PostgreSQL",
  "User-owned data control plane with fully modular services",
];
const currentSlide = ref(0);
let slideTimer: number | null = null;

function nextSlide() {
  currentSlide.value = (currentSlide.value + 1) % slides.length;
}

function setSlide(index: number) {
  currentSlide.value = index;
  resetTimer();
}

function resetTimer() {
  if (slideTimer !== null) {
    clearInterval(slideTimer);
  }
  slideTimer = window.setInterval(nextSlide, 4500);
}

onMounted(() => {
  resetTimer();
});

onUnmounted(() => {
  if (slideTimer !== null) {
    clearInterval(slideTimer);
  }
});

async function handleSignIn() {
  errorMessage.value = "";
  isLoading.value = true;
  const result = await auth.signIn(email.value, password.value);
  isLoading.value = false;
  if (result.error) {
    errorMessage.value = result.error.detail;
    return;
  }
  if (result.response.mfa_required) {
    isMFA.value = true;
    return;
  }
  navigateTo("/", { replace: true });
}

async function handleMFA() {
  errorMessage.value = "";
  isLoading.value = true;
  const result = await auth.verifyMFA(mfaCode.value);
  isLoading.value = false;
  if (result.error) {
    errorMessage.value = result.error.detail;
    return;
  }
  navigateTo("/", { replace: true });
}
</script>

<template>
  <div
    class="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-background p-4 sm:p-6 text-foreground"
  >
    <!-- SVG Stipple Noise Pattern (Strict Primary Color) -->
    <svg class="sr-only" aria-hidden="true">
      <defs>
        <pattern id="stipplePrimary" width="8" height="8" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r="1.2" fill="#fff" opacity="0.35" />
          <circle cx="6" cy="6" r="1.1" fill="#000" opacity="0.35" />
          <circle cx="6" cy="2" r="0.8" fill="#fff" opacity="0.25" />
          <circle cx="2" cy="6" r="1.1" fill="#000" opacity="0.35" />
        </pattern>
        <radialGradient id="sphereShading" cx="35%" cy="30%" r="70%">
          <stop offset="0%" stop-color="#fff" stop-opacity="0.55" />
          <stop offset="35%" stop-color="#fff" stop-opacity="0.08" />
          <stop offset="70%" stop-color="#000" stop-opacity="0.4" />
          <stop offset="100%" stop-color="#000" stop-opacity="0.8" />
        </radialGradient>
      </defs>
    </svg>

    <!-- Background: Full-Screen 3x3 Animated Editorial Grid (Layr Services) -->
    <div
      class="absolute inset-0 grid grid-cols-3 grid-rows-3 divide-x divide-y divide-default/40 overflow-hidden opacity-85"
    >
      <!-- Cell 1: layr/data — Relational Database Cylinder & Table Grid (Top Left) -->
      <div class="flex items-center justify-center p-6">
        <div class="art-float translate-y-32 translate-x-6">
          <svg viewBox="0 0 200 200" class="size-56 text-primary">
            <!-- Top Cylinder Cap -->
            <ellipse cx="100" cy="45" rx="70" ry="24" fill="currentColor" opacity="0.95" />
            <ellipse cx="100" cy="45" rx="70" ry="24" fill="url(#stipplePrimary)" />
            <!-- Cylinder Body Tier 1 -->
            <path
              d="M30 45 V85 C30 98 61 109 100 109 C139 109 170 98 170 85 V45 Z"
              fill="currentColor"
              opacity="0.75"
            />
            <path
              d="M30 45 V85 C30 98 61 109 100 109 C139 109 170 98 170 85 V45 Z"
              fill="url(#stipplePrimary)"
            />
            <!-- Cylinder Body Tier 2 -->
            <path
              d="M30 90 V130 C30 143 61 154 100 154 C139 154 170 143 170 130 V90 Z"
              fill="currentColor"
              opacity="0.5"
            />
            <path
              d="M30 90 V130 C30 143 61 154 100 154 C139 154 170 143 170 130 V90 Z"
              fill="url(#stipplePrimary)"
            />
          </svg>
        </div>
      </div>

      <!-- Cell 2: Empty Grid Space (Top Center) -->
      <div class="flex items-center justify-center p-6" />

      <!-- Cell 3: layr/auth — 3D Volumetric Sphere Ball with Orbital Rings (Top Right) -->
      <div class="flex items-center justify-center p-6">
        <div class="art-spin">
          <svg viewBox="0 0 200 200" class="size-72 text-primary overflow-visible">
            <!-- Outer Dashed Circular Halo Ring -->
            <circle
              cx="100"
              cy="100"
              r="86"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-dasharray="12 8"
              opacity="0.3"
            />

            <!-- Solid Base Ball -->
            <circle cx="100" cy="100" r="68" fill="currentColor" opacity="0.95" />

            <!-- Stipple Noise Texture Layer -->
            <circle cx="100" cy="100" r="68" fill="url(#stipplePrimary)" />

            <!-- 3D Spherical Volume Gradient Overlay -->
            <circle cx="100" cy="100" r="68" fill="url(#sphereShading)" />

            <!-- Specular Light Highlights -->
            <circle cx="76" cy="72" r="16" fill="#fff" opacity="0.3" />
            <circle cx="72" cy="68" r="6" fill="#fff" opacity="0.65" />
          </svg>
        </div>
      </div>

      <!-- Cell 4: Empty Grid Space (Middle Left) -->
      <div class="flex items-center justify-center p-6" />

      <!-- Cell 5: Empty Grid Space Behind Centered Form Card (Center) -->
      <div class="flex items-center justify-center p-6" />

      <!-- Cell 6: layr/storage — Morphing 2D Diamond into 3D Isometric Storage Box (Middle Right) -->
      <div class="flex items-center justify-center p-6">
        <div class="art-morph-rotate">
          <svg viewBox="0 0 200 200" class="size-56 text-primary overflow-visible">
            <!-- Top Facet (Morphs from 2D Diamond to Isometric Top Lid) -->
            <path d="M100 35 L165 100 L100 165 L35 100 Z" fill="currentColor" opacity="0.95">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M100 35 L165 100 L100 165 L35 100 Z; M100 30 L160 65 L100 100 L40 65 Z; M100 30 L160 65 L100 100 L40 65 Z; M100 35 L165 100 L100 165 L35 100 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>
            <path d="M100 35 L165 100 L100 165 L35 100 Z" fill="url(#stipplePrimary)">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M100 35 L165 100 L100 165 L35 100 Z; M100 30 L160 65 L100 100 L40 65 Z; M100 30 L160 65 L100 100 L40 65 Z; M100 35 L165 100 L100 165 L35 100 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>

            <!-- Left Facet (Unfolds from 2D flat into 3D Left Wall) -->
            <path d="M35 100 L100 165 L100 165 L35 100 Z" fill="currentColor" opacity="0.55">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M35 100 L100 165 L100 165 L35 100 Z; M40 65 L100 100 L100 165 L40 130 Z; M40 65 L100 100 L100 165 L40 130 Z; M35 100 L100 165 L100 165 L35 100 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>
            <path d="M35 100 L100 165 L100 165 L35 100 Z" fill="url(#stipplePrimary)">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M35 100 L100 165 L100 165 L35 100 Z; M40 65 L100 100 L100 165 L40 130 Z; M40 65 L100 100 L100 165 L40 130 Z; M35 100 L100 165 L100 165 L35 100 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>

            <!-- Right Facet (Unfolds from 2D flat into 3D Right Wall) -->
            <path d="M100 165 L165 100 L165 100 L100 165 Z" fill="currentColor" opacity="0.75">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M100 165 L165 100 L165 100 L100 165 Z; M100 100 L160 65 L160 130 L100 165 Z; M100 100 L160 65 L160 130 L100 165 Z; M100 165 L165 100 L165 100 L100 165 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>
            <path d="M100 165 L165 100 L165 100 L100 165 Z" fill="url(#stipplePrimary)">
              <animate
                attributeName="d"
                dur="7s"
                repeatCount="indefinite"
                values="M100 165 L165 100 L165 100 L100 165 Z; M100 100 L160 65 L160 130 L100 165 Z; M100 100 L160 65 L160 130 L100 165 Z; M100 165 L165 100 L165 100 L100 165 Z"
                keyTimes="0; 0.35; 0.65; 1"
                calcMode="spline"
                keySplines="0.4 0 0.2 1; 0 0 1 1; 0.4 0 0.2 1"
              />
            </path>
          </svg>
        </div>
      </div>

      <!-- Cell 7 & 8: layr/scheduler — Stepped Job Dispatcher & Interactive Headline Carousel (Bottom Row) -->
      <div class="col-span-2 flex flex-col justify-between p-6 sm:p-8">
        <!-- Stepped Task Timeline Block -->
        <div class="art-step mb-6">
          <svg viewBox="0 0 220 180" class="size-56 text-primary">
            <path
              d="M0 0 H200 V45 H150 V90 H100 V135 H50 V180 H0 Z"
              fill="currentColor"
              opacity="0.9"
            />
            <path d="M0 0 H200 V45 H150 V90 H100 V135 H50 V180 H0 Z" fill="url(#stipplePrimary)" />
          </svg>
        </div>

        <!-- Headline & Interactive Slider Indicators -->
        <div class="space-y-4">
          <div class="min-h-19 flex items-center">
            <Transition name="slide-fade" mode="out-in">
              <h2
                :key="currentSlide"
                class="text-2xl sm:text-3xl font-extrabold tracking-tight text-foreground max-w-lg leading-tight"
              >
                {{ slides[currentSlide] }}
              </h2>
            </Transition>
          </div>
          <div class="flex items-center gap-2">
            <button
              v-for="(_, index) in slides"
              :key="index"
              type="button"
              :aria-label="`Go to slide ${index + 1}`"
              class="h-1.5 rounded-full transition-all duration-300 cursor-pointer pointer-events-auto border-0 p-0"
              :class="
                currentSlide === index ? 'w-7 bg-primary' : 'w-2 bg-accented/50 hover:bg-accented'
              "
              @click="setSlide(index)"
            />
          </div>
        </div>
      </div>

      <!-- Cell 9: Empty Grid Space (Bottom Right) -->
      <div class="flex items-center justify-center p-6" />
    </div>

    <!-- Centered Form Floating Card Container -->
    <div class="relative z-10 w-full max-w-sm">
      <UCard class="shadow-2xl">
        <div class="mb-4">
          <div class="flex items-center gap-2 mb-3">
            <img src="/icon-glyph-light.png" width="32" height="32" class="w-7 h-7 dark:hidden" />
            <img
              src="/icon-glyph-dark.png"
              width="32"
              height="32"
              class="w-7 h-7 hidden dark:block"
            />
          </div>
          <h2 class="text-xl font-bold tracking-tight text-highlighted">
            {{ isMFA ? "Two-factor authentication" : "Sign in" }}
          </h2>
          <p class="text-sm text-muted mt-1">
            {{
              isMFA
                ? "Enter the 6-digit code from your authenticator app"
                : "Enter your credentials to access the console"
            }}
          </p>
        </div>

        <!-- Error Alert -->
        <UAlert
          v-if="errorMessage"
          color="error"
          variant="subtle"
          icon="i-lucide-alert-circle"
          :title="errorMessage"
          class="mb-4"
        />

        <!-- MFA Form -->
        <form v-if="isMFA" class="space-y-4" @submit.prevent="handleMFA">
          <UFormField label="Authentication code" name="totp" class="w-full">
            <div class="py-2">
              <UPinInput
                v-model="mfaPin"
                type="number"
                otp
                :length="6"
                placeholder="○"
                autofocus
                @complete="handleMFA"
              />
            </div>
          </UFormField>

          <UButton
            type="submit"
            label="Verify code"
            :disabled="mfaCode.length !== 6"
            :loading="isLoading"
            block
          />

          <UButton
            type="button"
            label="Back to sign in"
            variant="ghost"
            size="sm"
            block
            @click="
              () => {
                isMFA = false;
                errorMessage = '';
                mfaPin = [];
              }
            "
          />
        </form>

        <!-- Email & Password Form -->
        <form v-else class="space-y-4" @submit.prevent="handleSignIn">
          <UFormField label="Email" name="email" required>
            <UInput
              id="signin-email"
              v-model="email"
              type="email"
              autocomplete="email"
              placeholder="user@example.com"
              icon="i-lucide-mail"
              required
              autofocus
              class="w-full"
            />
          </UFormField>

          <UFormField label="Password" name="password" required>
            <UInput
              id="signin-password"
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="current-password"
              placeholder="••••••••••••"
              icon="i-lucide-lock"
              required
              class="w-full"
              :ui="{ trailing: 'pe-1' }"
            >
              <template #trailing>
                <UButton
                  color="neutral"
                  variant="link"
                  :icon="showPassword ? 'i-lucide-eye-off' : 'i-lucide-eye'"
                  :aria-label="showPassword ? 'Hide password' : 'Show password'"
                  tabindex="-1"
                  @click="showPassword = !showPassword"
                />
              </template>
            </UInput>
          </UFormField>

          <UButton type="submit" label="Sign in" :loading="isLoading" block class="mt-2" />
        </form>
      </UCard>
    </div>
  </div>
</template>

<style scoped>
@keyframes artFloat {
  0%,
  100% {
    transform: translateY(0) scale(1);
  }
  50% {
    transform: translateY(-14px) scale(1.02);
  }
}

@keyframes artSpin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@keyframes artPulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.9;
  }
  50% {
    transform: scale(1.06);
    opacity: 1;
  }
}

@keyframes morphRotate {
  0%,
  100% {
    transform: translateY(0) rotate(-14deg) scale(0.96);
  }
  50% {
    transform: translateY(-10px) rotate(14deg) scale(1.04);
  }
}

@keyframes artStep {
  0%,
  100% {
    transform: translate(0, 0);
  }
  50% {
    transform: translate(5px, -5px);
  }
}

.art-float {
  animation: artFloat 6s ease-in-out infinite;
}

.art-spin {
  animation: artSpin 32s linear infinite;
  transform-origin: center;
}

.art-pulse {
  animation: artPulse 5s ease-in-out infinite;
}

.art-morph-rotate {
  animation: morphRotate 7s ease-in-out infinite;
  transform-origin: center;
}

.art-step {
  animation: artStep 7s ease-in-out infinite;
}

.slide-fade-enter-active,
.slide-fade-leave-active {
  transition:
    opacity 0.35s ease,
    transform 0.35s ease;
}

.slide-fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
