export const prerender = false;
export const ssr = false;
export const csr = true;

import { goto } from "$app/navigation";
import axios from "axios";
import { baseURL, getToken } from "@/lib/util/api";
import { notify } from "@/lib/util/notify";
import { clearWatcharrData } from "@/lib/logout";

axios.interceptors.request.use(
	(config) => {
		if (!config.baseURL) {
			config.baseURL = baseURL;

			// Only want to set auth header if requesting to our backend.
			const token = getToken();
			// Don't require token check if going to auth route (login/register)
			// or public routes accessible by guests (shared lists)
			const isPublicRoute =
				config.url?.includes("/auth") ||
				config.url?.match(/^\/watched\/\d+\//) ||
				config.url?.match(/^\/user\/public\//);
			if (!token && !isPublicRoute) {
				console.error("No token, going to login. Endpoint:", config.url);
				goto("/login?again=1");
				throw new axios.Cancel("No auth token found");
			}
			if (token) {
				config.headers.set("Authorization", token);
			}
		}

		return config;
	},
	(error) => {
		return Promise.reject(error);
	},
);

axios.interceptors.response.use(
	(response) => {
		return response;
	},
	(error) => {
		if (error.response?.status === 401) {
			console.error("Recieved 401 response, going to login.");
			notify({ text: "Request Authorization Failed!", type: "error" });
			clearWatcharrData();
			goto("/login?again=1");
		}
		return Promise.reject(error);
	},
);
