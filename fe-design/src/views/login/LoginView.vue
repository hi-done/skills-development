<template>
  <div class="login-page">
    <!-- 背景 -->
    <div class="background">
      <div class="grid"></div>

      <div class="orb orb-1"></div>
      <div class="orb orb-2"></div>
      <div class="orb orb-3"></div>

      <div class="scan-line"></div>

      <div class="bg-text bg-text-1">SYSTEM // 01</div>
      <div class="bg-text bg-text-2">ACCESS TERMINAL</div>
    </div>

    <!-- 中央内容 -->
    <main class="main-content">

      <!-- Logo / 标题 -->
      <div class="brand">
        <div class="brand-symbol">
          <span></span>
          <span></span>
          <span></span>
        </div>

        <div class="brand-title">
          <strong>NEXUS</strong>
          <small>CONTROL SYSTEM</small>
        </div>
      </div>

      <!-- 赛车启动按钮 -->
      <button
        class="start-button"
        @click="openLogin"
        aria-label="Sign in"
      >
        <span class="button-glow"></span>

        <span class="button-ring ring-1"></span>
        <span class="button-ring ring-2"></span>

        <span class="button-inner">
          <span class="power-icon">
            <i></i>
          </span>

          <span class="button-text">
            SIGN IN
          </span>
        </span>

        <span class="button-bolt bolt-1"></span>
        <span class="button-bolt bolt-2"></span>
        <span class="button-bolt bolt-3"></span>
        <span class="button-bolt bolt-4"></span>
      </button>

      <div class="hint">
        PRESS TO INITIALIZE
      </div>

    </main>

    <!-- 遮罩 -->
    <transition name="fade">
      <div
        v-if="showLogin || showRegister"
        class="modal-overlay"
        @click.self="closeAll"
      >
        <!-- 登录 -->
        <transition name="modal" mode="out-in">

          <section
            v-if="showLogin"
            key="login"
            class="cyber-card"
          >
            <!-- 不规则闪光 -->
            <span class="spark spark-1"></span>
            <span class="spark spark-2"></span>
            <span class="spark spark-3"></span>
            <span class="spark spark-4"></span>

            <div class="card-top">
              <div>
                <span class="card-label">SYSTEM ACCESS</span>
                <h1>Welcome Back</h1>
              </div>

              <button
                class="close-btn"
                @click="closeAll"
              >
                ×
              </button>
            </div>

            <div class="cyber-line"></div>

            <form @submit.prevent="login">
              <!-- 用户名 -->
              <div class="input-group">
                <label>USERNAME</label>

                <div class="input-wrapper">
                  <span class="input-icon">◈</span>

                  <input
                    v-model="loginForm.username"
                    type="text"
                    placeholder="Enter username"
                    autocomplete="username"
                  />

                  <span class="input-status"></span>
                </div>
              </div>

              <!-- 密码 -->
              <div class="input-group">
                <label>PASSWORD</label>

                <div class="input-wrapper">
                  <span class="input-icon">◆</span>

                  <input
                    v-model="loginForm.password"
                    :type="showPassword ? 'text' : 'password'"
                    placeholder="Enter password"
                    autocomplete="current-password"
                  />

                  <button
                    type="button"
                    class="password-toggle"
                    @click="showPassword = !showPassword"
                  >
                    {{ showPassword ? 'HIDE' : 'SHOW' }}
                  </button>
                </div>
              </div>

              <!-- 登录按钮 -->
              <button
                class="cyber-submit"
                type="submit"
              >
                <span>AUTHENTICATE</span>
                <i>→</i>
              </button>
            </form>

            <!-- 注册 -->
            <div class="register-tip">
              <span>NO ACCOUNT?</span>

              <button @click="openRegister">
                CREATE ACCOUNT
              </button>
            </div>

            <div class="card-footer">
              <span>SECURE CONNECTION</span>
              <span class="secure-dot"></span>
              <span>ENCRYPTED</span>
            </div>
          </section>

          <!-- 注册 -->
          <section
            v-else
            key="register"
            class="cyber-card register-card"
          >
            <span class="spark spark-1"></span>
            <span class="spark spark-2"></span>
            <span class="spark spark-3"></span>
            <span class="spark spark-4"></span>

            <div class="card-top">
              <div>
                <span class="card-label">NEW USER REGISTRATION</span>
                <h1>Create Account</h1>
              </div>

              <button
                class="close-btn"
                @click="closeAll"
              >
                ×
              </button>
            </div>

            <div class="cyber-line"></div>

            <form @submit.prevent="register">

              <!-- 用户名 -->
              <div class="input-group">
                <label>USERNAME</label>

                <div class="input-wrapper">
                  <span class="input-icon">◈</span>

                  <input
                    v-model="registerForm.username"
                    type="text"
                    placeholder="Create username"
                    autocomplete="username"
                  />
                </div>
              </div>

              <!-- 密码 -->
              <div class="input-group">
                <label>PASSWORD</label>

                <div class="input-wrapper">
                  <span class="input-icon">◆</span>

                  <input
                    v-model="registerForm.password"
                    type="password"
                    placeholder="Create password"
                    autocomplete="new-password"
                  />
                </div>
              </div>

              <!-- 重复密码 -->
              <div class="input-group">
                <label>CONFIRM PASSWORD</label>

                <div class="input-wrapper">
                  <span class="input-icon">◆</span>

                  <input
                    v-model="registerForm.confirmPassword"
                    type="password"
                    placeholder="Repeat password"
                    autocomplete="new-password"
                  />
                </div>
              </div>

              <!-- 邀请码 -->
              <div class="input-group">
                <label>INVITATION CODE</label>

                <div class="input-wrapper">
                  <span class="input-icon">◇</span>

                  <input
                    v-model="registerForm.invitationCode"
                    type="text"
                    placeholder="Optional invitation code"
                  />
                </div>
              </div>

              <!-- 注册 -->
              <button
                class="cyber-submit"
                type="submit"
              >
                <span>INITIALIZE ACCOUNT</span>
                <i>→</i>
              </button>
            </form>

            <div class="register-tip">
              <span>ALREADY REGISTERED?</span>

              <button @click="openLogin">
                RETURN TO LOGIN
              </button>
            </div>

            <div class="card-footer">
              <span>REGISTRATION NODE</span>
              <span class="secure-dot"></span>
              <span>READY</span>
            </div>
          </section>

        </transition>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'

/* =========================
 * Modal
 * ========================= */

const showLogin = ref(false)
const showRegister = ref(false)

const showPassword = ref(false)

/* =========================
 * Form
 * ========================= */

const loginForm = reactive({
  username: '',
  password: ''
})

const registerForm = reactive({
  username: '',
  password: '',
  confirmPassword: '',
  invitationCode: ''
})

/* =========================
 * Actions
 * ========================= */

function openLogin() {
  showRegister.value = false
  showLogin.value = true
}

function openRegister() {
  showLogin.value = false

  // 下一帧打开注册弹窗
  requestAnimationFrame(() => {
    showRegister.value = true
  })
}

function closeAll() {
  showLogin.value = false
  showRegister.value = false
}

/* =========================
 * Login
 * ========================= */

function login() {
  if (!loginForm.username || !loginForm.password) {
    return
  }

  console.log('Login:', {
    username: loginForm.username,
    password: loginForm.password
  })

  // TODO:
  // 在这里调用你的登录 API

  closeAll()
}

/* =========================
 * Register
 * ========================= */

function register() {
  if (
    !registerForm.username ||
    !registerForm.password ||
    !registerForm.confirmPassword
  ) {
    return
  }

  if (
    registerForm.password !==
    registerForm.confirmPassword
  ) {
    alert('PASSWORD DOES NOT MATCH')
    return
  }

  console.log('Register:', registerForm)

  // TODO:
  // 在这里调用你的注册 API

  closeAll()
}
</script>

<style scoped>
/* =========================================================
   Global
========================================================= */

* {
  box-sizing: border-box;
}

.login-page {
  position: relative;
  width: 100%;
  min-height: 100vh;
  overflow: hidden;

  display: flex;
  align-items: center;
  justify-content: center;

  background:
    radial-gradient(
      circle at 50% 45%,
      rgba(35, 40, 70, 0.8),
      transparent 45%
    ),
    #050609;

  color: #ffffff;

  font-family:
    Inter,
    "Segoe UI",
    Arial,
    sans-serif;
}

/* =========================================================
   Background
========================================================= */

.background {
  position: absolute;
  inset: 0;
  overflow: hidden;

  pointer-events: none;
}

.grid {
  position: absolute;
  inset: -50%;

  background-image:
    linear-gradient(
      rgba(0, 255, 255, 0.045) 1px,
      transparent 1px
    ),
    linear-gradient(
      90deg,
      rgba(0, 255, 255, 0.045) 1px,
      transparent 1px
    );

  background-size: 50px 50px;

  transform:
    perspective(500px)
    rotateX(60deg)
    translateY(100px);

  opacity: 0.65;
}

.orb {
  position: absolute;

  width: 450px;
  height: 450px;

  border-radius: 50%;

  filter: blur(100px);

  opacity: 0.12;
}

.orb-1 {
  top: -200px;
  left: -150px;
  background: #ff1744;
}

.orb-2 {
  right: -200px;
  top: 20%;
  background: #00e5ff;
}

.orb-3 {
  bottom: -250px;
  left: 35%;
  background: #7c4dff;
}

.scan-line {
  position: absolute;

  width: 100%;
  height: 1px;

  background:
    linear-gradient(
      90deg,
      transparent,
      rgba(0, 255, 255, 0.35),
      transparent
    );

  animation: scan 5s linear infinite;

  opacity: 0.4;
}

@keyframes scan {
  0% {
    top: -10%;
  }

  100% {
    top: 110%;
  }
}

.bg-text {
  position: absolute;

  color: rgba(255, 255, 255, 0.025);

  font-weight: 900;

  letter-spacing: 15px;

  white-space: nowrap;
}

.bg-text-1 {
  top: 15%;
  left: 5%;
  font-size: 80px;
}

.bg-text-2 {
  right: -80px;
  bottom: 15%;
  font-size: 60px;
}

/* =========================================================
   Main
========================================================= */

.main-content {
  position: relative;

  z-index: 2;

  display: flex;
  flex-direction: column;
  align-items: center;
}

/* =========================================================
   Brand
========================================================= */

.brand {
  display: flex;
  align-items: center;
  gap: 14px;

  margin-bottom: 55px;
}

.brand-symbol {
  width: 44px;
  height: 44px;

  position: relative;

  transform: skew(-15deg);
}

.brand-symbol span {
  position: absolute;

  width: 28px;
  height: 5px;

  right: 0;

  background: #ff1744;

  box-shadow:
    0 0 10px rgba(255, 23, 68, 0.8);
}

.brand-symbol span:nth-child(1) {
  top: 7px;
}

.brand-symbol span:nth-child(2) {
  top: 19px;
  width: 35px;
}

.brand-symbol span:nth-child(3) {
  top: 31px;
  width: 20px;
}

.brand-title {
  display: flex;
  flex-direction: column;
}

.brand-title strong {
  font-size: 22px;
  letter-spacing: 8px;
}

.brand-title small {
  margin-top: 4px;

  font-size: 8px;

  letter-spacing: 4px;

  color: #7b8496;
}

/* =========================================================
   Racing Start Button
========================================================= */

.start-button {
  position: relative;

  width: 190px;
  height: 190px;

  border: none;

  border-radius: 50%;

  cursor: pointer;

  background: transparent;

  filter:
    drop-shadow(
      0 20px 40px rgba(0, 0, 0, 0.7)
    );

  transition:
    transform 0.15s ease;
}

.start-button:hover {
  transform: scale(1.05);
}

.start-button:active {
  transform:
    scale(0.94)
    translateY(5px);
}

/* Outer metallic rings */

.button-ring {
  position: absolute;

  inset: 0;

  border-radius: 50%;

  pointer-events: none;
}

.ring-1 {
  border:
    8px solid #22252c;

  box-shadow:
    inset 0 2px 3px #6c7078,
    inset 0 -4px 5px #050609,
    0 2px 5px #000;
}

.ring-2 {
  inset: 12px;

  border:
    2px solid #41454d;

  box-shadow:
    0 0 0 2px #0b0c10;
}

/* Core */

.button-inner {
  position: absolute;

  inset: 30px;

  border-radius: 50%;

  display: flex;
  flex-direction: column;

  justify-content: center;
  align-items: center;

  gap: 12px;

  background:
    radial-gradient(
      circle at 35% 25%,
      #ff5068,
      #d9002f 35%,
      #650014 100%
    );

  border:
    4px solid #ff3b57;

  box-shadow:
    inset 0 5px 10px rgba(255, 255, 255, 0.25),
    inset 0 -15px 20px rgba(0, 0, 0, 0.45),
    0 0 30px rgba(255, 0, 60, 0.4);

  transition: box-shadow 0.15s;
}

.start-button:active .button-inner {
  box-shadow:
    inset 0 10px 20px rgba(0, 0, 0, 0.5),
    0 0 10px rgba(255, 0, 60, 0.3);
}

.button-text {
  font-size: 15px;

  font-weight: 900;

  letter-spacing: 3px;

  text-shadow:
    0 2px 2px rgba(0, 0, 0, 0.7);
}

/* Power icon */

.power-icon {
  width: 28px;
  height: 28px;

  border:
    3px solid white;

  border-top-color: transparent;

  border-radius: 50%;

  position: relative;
}

.power-icon i {
  position: absolute;

  width: 3px;
  height: 14px;

  left: 50%;
  top: -5px;

  transform: translateX(-50%);

  background: white;

  border-radius: 2px;
}

/* Bolts */

.button-bolt {
  position: absolute;

  width: 5px;
  height: 13px;

  background: #6f737a;

  border-radius: 2px;

  box-shadow:
    inset 1px 0 #aaa;
}

.bolt-1 {
  top: 17px;
  left: 92px;
}

.bolt-2 {
  bottom: 17px;
  left: 92px;
}

.bolt-3 {
  left: 17px;
  top: 88px;

  transform: rotate(90deg);
}

.bolt-4 {
  right: 17px;
  top: 88px;

  transform: rotate(90deg);
}

.button-glow {
  position: absolute;

  inset: -20px;

  border-radius: 50%;

  background:
    radial-gradient(
      circle,
      rgba(255, 0, 50, 0.2),
      transparent 65%
    );

  animation: buttonPulse 2s infinite;
}

@keyframes buttonPulse {
  0%,
  100% {
    transform: scale(0.9);
    opacity: 0.5;
  }

  50% {
    transform: scale(1.15);
    opacity: 1;
  }
}

.hint {
  margin-top: 32px;

  font-size: 9px;

  letter-spacing: 5px;

  color: #586070;
}

/* =========================================================
   Modal
========================================================= */

.modal-overlay {
  position: fixed;

  inset: 0;

  z-index: 100;

  display: flex;

  justify-content: center;
  align-items: center;

  padding: 20px;

  background:
    rgba(1, 3, 8, 0.78);

  backdrop-filter: blur(12px);
}

/* =========================================================
   Cyber Card
========================================================= */

.cyber-card {
  position: relative;

  width: 440px;

  max-width: 100%;

  padding: 34px;

  overflow: hidden;

  background:
    linear-gradient(
      145deg,
      rgba(24, 27, 38, 0.97),
      rgba(8, 10, 16, 0.98)
    );

  border:
    1px solid rgba(0, 229, 255, 0.25);

  box-shadow:
    0 30px 80px rgba(0, 0, 0, 0.75),
    0 0 40px rgba(0, 229, 255, 0.08);

  clip-path: polygon(
    0 14px,
    14px 0,
    calc(100% - 25px) 0,
    100% 25px,
    100% calc(100% - 14px),
    calc(100% - 14px) 100%,
    25px 100%,
    0 calc(100% - 25px)
  );
}

/* 顶部装饰 */

.cyber-card::before {
  content: "";

  position: absolute;

  top: 0;
  left: 15%;

  width: 45%;

  height: 2px;

  background:
    linear-gradient(
      90deg,
      transparent,
      #00e5ff,
      transparent
    );

  box-shadow:
    0 0 15px #00e5ff;
}

.cyber-card::after {
  content: "";

  position: absolute;

  right: 0;
  bottom: 30px;

  width: 100px;
  height: 1px;

  background:
    linear-gradient(
      90deg,
      transparent,
      #ff1744
    );
}

/* =========================================================
   Card Header
========================================================= */

.card-top {
  display: flex;

  justify-content: space-between;

  align-items: flex-start;
}

.card-label {
  display: block;

  margin-bottom: 8px;

  color: #00e5ff;

  font-size: 9px;

  font-weight: 700;

  letter-spacing: 4px;
}

.card-top h1 {
  margin: 0;

  font-size: 28px;

  font-weight: 700;

  letter-spacing: 1px;
}

.close-btn {
  width: 32px;
  height: 32px;

  border: 1px solid #343944;

  background: #11131a;

  color: #777f90;

  font-size: 22px;

  cursor: pointer;

  transition: 0.2s;
}

.close-btn:hover {
  color: #ff1744;

  border-color: #ff1744;

  box-shadow:
    0 0 12px rgba(255, 23, 68, 0.3);
}

.cyber-line {
  height: 1px;

  margin: 25px 0;

  background:
    linear-gradient(
      90deg,
      #00e5ff,
      rgba(0, 229, 255, 0.1),
      transparent
    );
}

/* =========================================================
   Form
========================================================= */

.input-group {
  margin-bottom: 20px;
}

.input-group label {
  display: block;

  margin-bottom: 8px;

  color: #7f8797;

  font-size: 9px;

  font-weight: 700;

  letter-spacing: 3px;
}

.input-wrapper {
  position: relative;

  height: 48px;
}

.input-wrapper input {
  width: 100%;
  height: 100%;

  padding:
    0 45px
    0 42px;

  outline: none;

  border:
    1px solid #292d37;

  background:
    rgba(4, 6, 11, 0.8);

  color: #fff;

  font-size: 13px;

  transition: 0.2s;

  clip-path: polygon(
    0 0,
    calc(100% - 10px) 0,
    100% 10px,
    100% 100%,
    10px 100%,
    0 calc(100% - 10px)
  );
}

.input-wrapper input:focus {
  border-color: #00e5ff;

  box-shadow:
    inset 0 0 20px rgba(0, 229, 255, 0.04),
    0 0 12px rgba(0, 229, 255, 0.1);
}

.input-wrapper input::placeholder {
  color: #424957;
}

.input-icon {
  position: absolute;

  z-index: 2;

  left: 15px;
  top: 50%;

  transform: translateY(-50%);

  color: #00e5ff;

  font-size: 13px;
}

.input-status {
  position: absolute;

  right: 12px;
  top: 50%;

  width: 5px;
  height: 5px;

  transform: translateY(-50%);

  border-radius: 50%;

  background: #00e5ff;

  box-shadow:
    0 0 8px #00e5ff;
}

.password-toggle {
  position: absolute;

  z-index: 2;

  right: 12px;
  top: 50%;

  transform: translateY(-50%);

  border: none;

  background: transparent;

  color: #4f596a;

  font-size: 8px;

  letter-spacing: 1px;

  cursor: pointer;
}

.password-toggle:hover {
  color: #00e5ff;
}

/* =========================================================
   Submit
========================================================= */

.cyber-submit {
  position: relative;

  width: 100%;
  height: 52px;

  margin-top: 8px;

  border: 1px solid #00e5ff;

  background:
    linear-gradient(
      90deg,
      rgba(0, 229, 255, 0.12),
      rgba(0, 229, 255, 0.02)
    );

  color: #00e5ff;

  font-size: 11px;

  font-weight: 800;

  letter-spacing: 3px;

  cursor: pointer;

  display: flex;

  align-items: center;
  justify-content: space-between;

  padding: 0 18px;

  clip-path: polygon(
    0 0,
    calc(100% - 14px) 0,
    100% 14px,
    100% 100%,
    0 100%
  );

  transition: 0.25s;
}

.cyber-submit i {
  font-size: 20px;

  font-style: normal;

  transition: 0.2s;
}

.cyber-submit:hover {
  background:
    rgba(0, 229, 255, 0.16);

  box-shadow:
    0 0 20px rgba(0, 229, 255, 0.2);

  text-shadow:
    0 0 10px #00e5ff;
}

.cyber-submit:hover i {
  transform: translateX(5px);
}

.cyber-submit:active {
  transform: scale(0.98);
}

/* =========================================================
   Register
========================================================= */

.register-tip {
  display: flex;

  justify-content: center;

  gap: 8px;

  margin-top: 25px;

  font-size: 8px;

  letter-spacing: 2px;

  color: #596171;
}

.register-tip button {
  padding: 0;

  border: none;

  background: transparent;

  color: #ff4260;

  font-size: 8px;

  font-weight: 800;

  letter-spacing: 2px;

  cursor: pointer;
}

.register-tip button:hover {
  text-shadow:
    0 0 10px #ff1744;
}

/* =========================================================
   Footer
========================================================= */

.card-footer {
  display: flex;

  align-items: center;

  justify-content: center;

  gap: 7px;

  margin-top: 28px;

  color: #373d49;

  font-size: 7px;

  letter-spacing: 2px;
}

.secure-dot {
  width: 4px;
  height: 4px;

  border-radius: 50%;

  background: #00e5ff;

  box-shadow:
    0 0 7px #00e5ff;
}

/* =========================================================
   Sparks
========================================================= */

.spark {
  position: absolute;

  width: 30px;
  height: 1px;

  background: #00e5ff;

  box-shadow:
    0 0 8px #00e5ff;

  opacity: 0.6;
}

.spark-1 {
  top: 18%;
  right: -5px;

  transform: rotate(-35deg);

  animation: spark 3s infinite;
}

.spark-2 {
  top: 48%;
  left: -8px;

  transform: rotate(20deg);

  background: #ff1744;

  box-shadow:
    0 0 8px #ff1744;

  animation: spark 4s 0.5s infinite;
}

.spark-3 {
  bottom: 20%;
  right: 15px;

  width: 18px;

  transform: rotate(50deg);

  animation: spark 2.5s 1s infinite;
}

.spark-4 {
  top: 8px;
  left: 35%;

  width: 15px;

  transform: rotate(-15deg);

  background: #ff1744;

  box-shadow:
    0 0 8px #ff1744;

  animation: spark 3.5s 0.3s infinite;
}

@keyframes spark {
  0%,
  100% {
    opacity: 0.15;
    transform: scaleX(0.5) rotate(-20deg);
  }

  50% {
    opacity: 1;
    transform: scaleX(1.2) rotate(20deg);
  }
}

/* =========================================================
   Transitions
========================================================= */

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.modal-enter-active,
.modal-leave-active {
  transition:
    opacity 0.25s ease,
    transform 0.25s ease;
}

.modal-enter-from {
  opacity: 0;
  transform:
    scale(0.9)
    translateY(20px);
}

.modal-leave-to {
  opacity: 0;
  transform:
    scale(0.95)
    translateY(-10px);
}

/* =========================================================
   Mobile
========================================================= */

@media (max-width: 600px) {
  .brand {
    margin-bottom: 45px;
  }

  .brand-title strong {
    font-size: 18px;
  }

  .start-button {
    width: 165px;
    height: 165px;
  }

  .button-inner {
    inset: 26px;
  }

  .bg-text {
    display: none;
  }

  .cyber-card {
    width: 100%;

    padding: 25px 20px;
  }

  .card-top h1 {
    font-size: 23px;
  }

  .input-group {
    margin-bottom: 16px;
  }

  .register-card {
    max-height: 92vh;

    overflow-y: auto;
  }
}

@media (max-height: 700px) {
  .brand {
    margin-bottom: 25px;
  }

  .start-button {
    width: 145px;
    height: 145px;
  }

  .button-inner {
    inset: 23px;
  }

  .hint {
    margin-top: 20px;
  }
}
</style>