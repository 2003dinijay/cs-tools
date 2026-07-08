export const BACKEND_URL = import.meta.env.VITE_BACKEND_URL;

if (!BACKEND_URL) {
  throw new Error("VITE_BACKEND_URL is not defined");
}

export const USERS_ME_ENDPOINT = "/users/me";

export const CASES_SEARCH_ENDPOINT = "/cases/search";
export const CASE_DETAILS_ENDPOINT = (id: string) => `/cases/${id}`;
export const CASE_COMMENTS_SEARCH_ENDPOINT = (id: string) => `/cases/${id}/comments/search`;
