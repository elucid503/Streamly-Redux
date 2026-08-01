// Shared, low-concurrency fetch queue for proxied images.
// The browser will happily open dozens of <img> requests at once; Wikimedia (and our
// paced proxy) cannot. Queueing here keeps the grid calm and retries failures with backoff.

const maxConcurrent = 3;
const maxAttempts = 5;

type Task = {

  url: string;
  signal: AbortSignal;
  resolve: (blob: Blob) => void;
  reject: (error: unknown) => void;
  attempt: number;

};

let active = 0;
const pending: Task[] = [];

export function fetchQueuedImage(url: string, signal?: AbortSignal): Promise<Blob> {

  return new Promise((resolve, reject) => {

    if (signal?.aborted) {

      reject(signal.reason ?? new DOMException("Aborted", "AbortError"));
      return;

    }

    const task: Task = {

      url,
      signal: signal ?? new AbortController().signal,
      resolve,
      reject,
      attempt: 0,

    };

    const onAbort = () => {

      const index = pending.indexOf(task);

      if (index >= 0) {

        pending.splice(index, 1);

      }

      reject(task.signal.reason ?? new DOMException("Aborted", "AbortError"));

    };

    task.signal.addEventListener("abort", onAbort, { once: true });

    pending.push(task);
    pump();

  });

}

function pump() {

  while (active < maxConcurrent && pending.length > 0) {

    const task = pending.shift()!;

    if (task.signal.aborted) {

      continue;

    }

    active += 1;

    void run(task).finally(() => {

      active -= 1;
      pump();

    });

  }

}

async function run(task: Task) {

  try {

    const blob = await attemptFetch(task);
    task.resolve(blob);

  } catch (error) {

    if (task.signal.aborted || isAbort(error)) {

      task.reject(error);
      return;

    }

    task.attempt += 1;

    if (task.attempt >= maxAttempts) {

      task.reject(error);
      return;

    }

    const delay = retryDelayMs(task.attempt, error);

    try {

      await wait(delay, task.signal);

    } catch (waitError) {

      task.reject(waitError);
      return;

    }

    // Re-queue toward the front so retries aren't starved by brand-new logos.
    pending.unshift(task);

  }

}

async function attemptFetch(task: Task): Promise<Blob> {

  const response = await fetch(task.url, {

    signal: task.signal,
    // Avoid reusing a failed intermediate response if any cache sits between us and Go.
    cache: "no-cache",

  });

  if (!response.ok) {

    const retryAfter = response.headers.get("Retry-After");
    const error = new ImageFetchError(response.status, retryAfter);
    throw error;

  }

  const blob = await response.blob();

  if (blob.size === 0) {

    throw new ImageFetchError(502, null);

  }

  return blob;

}

export class ImageFetchError extends Error {

  status: number;
  retryAfterMs: number | null;

  constructor(status: number, retryAfter: string | null) {

    super(`image fetch failed with ${status}`);
    this.status = status;
    this.retryAfterMs = parseRetryAfterMs(retryAfter);

  }

}

function retryDelayMs(attempt: number, error: unknown): number {

  if (error instanceof ImageFetchError && error.retryAfterMs !== null) {

    return Math.min(error.retryAfterMs, 30_000);

  }

  // 400ms, 900ms, 1.8s, 3.5s, …
  return Math.min(400 * 2 ** (attempt - 1) + Math.random() * 200, 8_000);

}

function parseRetryAfterMs(value: string | null): number | null {

  if (!value) {

    return null;

  }

  const seconds = Number(value);

  if (Number.isFinite(seconds) && seconds >= 0) {

    return seconds * 1000;

  }

  const date = Date.parse(value);

  if (!Number.isNaN(date)) {

    return Math.max(0, date - Date.now());

  }

  return null;

}

function wait(ms: number, signal: AbortSignal): Promise<void> {

  return new Promise((resolve, reject) => {

    if (signal.aborted) {

      reject(signal.reason ?? new DOMException("Aborted", "AbortError"));
      return;

    }

    const timer = window.setTimeout(() => {

      signal.removeEventListener("abort", onAbort);
      resolve();

    }, ms);

    const onAbort = () => {

      window.clearTimeout(timer);
      reject(signal.reason ?? new DOMException("Aborted", "AbortError"));

    };

    signal.addEventListener("abort", onAbort, { once: true });

  });

}

function isAbort(error: unknown): boolean {

  return error instanceof DOMException && error.name === "AbortError";

}
