const config = window.EVENTBOOKER_CONFIG || {};

const state = {
  token: localStorage.getItem("eventbooker.accessToken") || "",
  user: null,
  events: [],
  selectedEvent: null,
};

const elements = {
  registerForm: document.getElementById("register-form"),
  loginForm: document.getElementById("login-form"),
  logoutButton: document.getElementById("logout-button"),
  createEventForm: document.getElementById("create-event-form"),
  refreshEventsButton: document.getElementById("refresh-events-button"),
  eventsList: document.getElementById("events-list"),
  eventDetails: document.getElementById("event-details"),
  profileDetails: document.getElementById("profile-details"),
  currentUserName: document.getElementById("current-user-name"),
  telegramLink: document.getElementById("telegram-link"),
  telegramHint: document.getElementById("telegram-hint"),
  telegramStatus: document.getElementById("telegram-status"),
  toast: document.getElementById("toast"),
};

init();

async function init() {
  bindEvents();
  updateUserUI();
  await loadEvents();

  if (state.token) {
    await loadMe();
  }
}

function bindEvents() {
  elements.registerForm.addEventListener("submit", onRegister);
  elements.loginForm.addEventListener("submit", onLogin);
  elements.logoutButton.addEventListener("click", onLogout);
  elements.createEventForm.addEventListener("submit", onCreateEvent);
  elements.refreshEventsButton.addEventListener("click", () => loadEvents(true));
  elements.eventsList.addEventListener("click", onEventCardAction);
}

function onEventCardAction(event) {
  const actionButton = event.target.closest("button[data-action][data-event-id]");
  if (!actionButton) {
    return;
  }

  const eventID = actionButton.dataset.eventId;
  if (!eventID) {
    showToast("Invalid event id", true);
    return;
  }

  if (actionButton.dataset.action === "details") {
    loadEventDetails(eventID);
    return;
  }

  if (actionButton.dataset.action === "book") {
    bookEvent(eventID);
  }
}

async function onRegister(event) {
  event.preventDefault();
  const formElement = event.currentTarget;
  const form = new FormData(formElement);

  try {
    await apiFetch("/auth/register", {
      method: "POST",
      body: JSON.stringify({
        name: form.get("name"),
        email: form.get("email"),
        password: form.get("password"),
      }),
    });
    showToast("Account created. You can sign in right away.");
    formElement.reset();
  } catch (error) {
    showToast(error.message, true);
  }
}

async function onLogin(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);

  try {
    const response = await apiFetch("/auth/login", {
      method: "POST",
      body: JSON.stringify({
        email: form.get("email"),
        password: form.get("password"),
      }),
    });

    state.token = response.access_token;
    localStorage.setItem("eventbooker.accessToken", state.token);
    state.user = response.user;
    updateUserUI();
    await loadEvents();
    showToast("You are logged in.");
  } catch (error) {
    showToast(error.message, true);
  }
}

async function onLogout() {
  try {
    await apiFetch("/auth/logout", { method: "POST" }, true);
  } catch (error) {
    showToast(error.message, true);
  }

  state.token = "";
  state.user = null;
  localStorage.removeItem("eventbooker.accessToken");
  updateUserUI();
  showToast("Logged out.");
}

async function onCreateEvent(event) {
  event.preventDefault();
  const formElement = event.currentTarget;
  const form = new FormData(formElement);

  try {
    const localDate = form.get("start_at");
    await apiFetch("/events", {
      method: "POST",
      body: JSON.stringify({
        title: form.get("title"),
        start_at: new Date(localDate).toISOString(),
        capacity: Number(form.get("capacity")),
        booking_ttl_seconds: Number(form.get("booking_ttl_seconds")),
        requires_payment: form.get("requires_payment") === "on",
      }),
    });

    formElement.reset();
    showToast("Event created.");
    await loadEvents();
  } catch (error) {
    showToast(error.message, true);
  }
}

async function loadMe() {
  try {
    const user = await apiFetch("/me", {}, true);
    state.user = user;
    updateUserUI();
  } catch (error) {
    state.token = "";
    state.user = null;
    localStorage.removeItem("eventbooker.accessToken");
    updateUserUI();
  }
}

async function loadEvents(showFeedback = false) {
  try {
    state.events = await apiFetch("/events");
    renderEvents();
    if (state.selectedEvent) {
      await loadEventDetails(state.selectedEvent.event.id || state.selectedEvent.id);
    }
    if (showFeedback) {
      showToast("Events refreshed.");
    }
  } catch (error) {
    showToast(error.message, true);
  }
}

async function loadEventDetails(eventID) {
  try {
    state.selectedEvent = await apiFetch(`/events/${eventID}`);
    renderEventDetails();
  } catch (error) {
    showToast(error.message, true);
  }
}

async function bookEvent(eventID) {
  if (!state.token) {
    showToast("Sign in before booking an event.", true);
    return;
  }

  try {
    await apiFetch(`/events/${eventID}/book`, { method: "POST" }, true);
    showToast("Booking created.");
    await loadEventDetails(eventID);
    await loadEvents();
  } catch (error) {
    showToast(error.message, true);
  }
}

async function confirmBooking(eventID) {
  if (!state.token) {
    showToast("Sign in before confirming a booking.", true);
    return;
  }

  try {
    await apiFetch(`/events/${eventID}/confirm`, { method: "POST" }, true);
    showToast("Booking confirmed.");
    await loadEventDetails(eventID);
    await loadEvents();
  } catch (error) {
    showToast(error.message, true);
  }
}

function renderEvents() {
  if (!state.events.length) {
    elements.eventsList.innerHTML = '<div class="card empty-state">No events yet. Create the first one.</div>';
    return;
  }

  elements.eventsList.innerHTML = state.events.map((event) => {
    const paymentLabel = event.requires_payment ? "Payment confirmation" : "Instant confirmation";

    return `
      <article class="event-card">
        <div class="event-topline">
          <div>
            <h3 class="event-title">${escapeHtml(event.title)}</h3>
            <p class="muted">Starts ${formatDate(event.start_at)}</p>
          </div>
          <span class="pill">${paymentLabel}</span>
        </div>
        <div class="event-meta muted">
          <span>Capacity: ${event.capacity}</span>
          <span>Booking TTL: ${event.booking_ttl_seconds}s</span>
        </div>
        <div class="details-actions">
          <button class="button button-secondary" type="button" data-action="details" data-event-id="${event.id}">View details</button>
          <button class="button button-primary" type="button" data-action="book" data-event-id="${event.id}">Book event</button>
        </div>
      </article>
    `;
  }).join("");

}

function renderEventDetails() {
  const details = state.selectedEvent;
  if (!details) {
    elements.eventDetails.className = "card details-card empty-state";
    elements.eventDetails.textContent = "Select an event to see capacity, bookings, and actions.";
    return;
  }

  elements.eventDetails.className = "card details-card";
  elements.eventDetails.innerHTML = `
    <div class="details-grid">
      <div>
        <h3>${escapeHtml(details.event.title)}</h3>
        <p class="muted">Starts ${formatDate(details.event.start_at)}</p>
      </div>
      <div class="details-stats">
        <div class="stat-card"><span class="muted">Free seats</span><strong>${details.free_seats}</strong></div>
        <div class="stat-card"><span class="muted">Pending</span><strong>${details.pending_count}</strong></div>
        <div class="stat-card"><span class="muted">Confirmed</span><strong>${details.confirmed_count}</strong></div>
      </div>
      <div class="details-actions">
        <button class="button button-primary" id="details-book-button" type="button">Book this event</button>
        <button class="button button-secondary" id="details-confirm-button" type="button">Confirm my booking</button>
      </div>
      <div>
        <h3>Recent bookings</h3>
        <div class="booking-list">
          ${details.bookings.length ? details.bookings.map((booking) => `
            <div class="booking-item">
              <span>Booking #${booking.id}</span>
              <span class="muted">User ${booking.user_id} - ${booking.status}</span>
            </div>
          `).join("") : '<div class="booking-item"><span class="muted">No bookings yet.</span></div>'}
        </div>
      </div>
    </div>
  `;

  document.getElementById("details-book-button").addEventListener("click", () => bookEvent(details.event.id));
  document.getElementById("details-confirm-button").addEventListener("click", () => confirmBooking(details.event.id));
}

function updateUserUI() {
  const user = state.user;
  const values = user ? [user.id, user.name, user.email, user.role] : ["-", "-", "-", "-"];

  elements.profileDetails.querySelectorAll("dd").forEach((dd, index) => {
    dd.textContent = values[index];
  });

  elements.currentUserName.textContent = user ? user.name : "Guest";
  elements.logoutButton.classList.toggle("hidden", !user);

  const botUsername = (config.telegramBotUsername || "").trim();
  if (!botUsername) {
    elements.telegramLink.href = "#";
    elements.telegramLink.setAttribute("aria-disabled", "true");
    elements.telegramStatus.textContent = "Bot username is not configured";
    elements.telegramHint.textContent = "Set app.notifier.telegram_bot_username to enable Telegram deep links.";
    return;
  }

  const baseURL = `https://t.me/${botUsername}`;
  if (user) {
    elements.telegramLink.href = `${baseURL}?start=${encodeURIComponent(user.id)}`;
    elements.telegramStatus.textContent = `Ready for /start ${user.id}`;
    elements.telegramHint.textContent = `Use this button to open Telegram and link notifications for user #${user.id}.`;
  } else {
    elements.telegramLink.href = baseURL;
    elements.telegramStatus.textContent = "Sign in to build your personal link";
    elements.telegramHint.textContent = "Sign in first to open the bot with your personal /start <user_id> link.";
  }
}

async function apiFetch(path, options = {}, auth = false) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  if (auth && state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }

  const response = await fetch(path, {
    credentials: "include",
    ...options,
    headers,
  });

  if (response.status === 204) {
    return null;
  }

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `Request failed with status ${response.status}`);
  }

  return data;
}

function formatDate(value) {
  return new Date(value).toLocaleString();
}

function showToast(message, isError = false) {
  elements.toast.textContent = message;
  elements.toast.classList.remove("hidden");
  elements.toast.style.background = isError ? "rgba(133, 39, 29, 0.94)" : "rgba(28, 24, 20, 0.92)";

  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => {
    elements.toast.classList.add("hidden");
  }, 3200);
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
