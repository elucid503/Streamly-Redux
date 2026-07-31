// The Discord SDK rejects with plain objects rather than Errors, which stringify to "[object Object]".
export function formatError(error: unknown): string {

  if (error instanceof Error) {

    return error.message;

  }

  if (typeof error === "object" && error !== null) {

    try {

      return JSON.stringify(error);

    } catch {

      return String(error);

    }

  }

  return String(error);

}
