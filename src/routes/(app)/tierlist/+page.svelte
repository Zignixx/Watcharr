<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import { toPng } from "html-to-image";
	import type {
		Tier,
		TierItem,
		Tierlist,
		CreateTierRequest,
		Watched,
		TierPreset,
		WatchedStatus,
		SupportedMedia,
	} from "@/types";
	import { baseURL } from "@/lib/util/api";
	import { notify } from "@/lib/util/notify";
	import { store } from "@/store.svelte";
	import { parseTokenPayload } from "@/lib/util/helpers";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import Modal from "@/lib/Modal.svelte";
	import PosterContextMenu from "@/lib/poster/PosterContextMenu.svelte";

	let tiers: Tier[] = $state([]);
	let untieredWatched: any[] = $state([]);
	let loading = $state(true);
	let editMode = $state(false);

	// Tierlist (collection) state
	let tierlists: Tierlist[] = $state([]);
	let activeTierlistId: number | null = $state(null);
	let showCreateTierlistModal = $state(false);
	let newTierlistName = $state("");
	let showRenameTierlistModal = $state(false);
	let renameTierlistName = $state("");

	let activeTierlist = $derived(tierlists.find((tl) => tl.id === activeTierlistId));

	let showAddTierModal = $state(false);
	let editingTier: Tier | null = $state(null);
	let dragItem: { watchedId: number; sourceTierId: number | null; sourceIndex: number } | null =
		$state(null);
	let dragOverTierId: number | null = $state(null);
	let dragOverIndex: number | null = $state(null);
	let saving = $state(false);
	let showPresetModal = $state(false);
	let userPresets: TierPreset[] = $state([]);
	let showSavePresetModal = $state(false);
	let savePresetName = $state("");
	let savePresetDesc = $state("");
	let tierlistContainer: HTMLDivElement | undefined = $state(undefined);
	let exporting = $state(false);
	let showGradientModal = $state(false);
	let selectedGradient: string | null = $state(null);
	let gradientReversed = $state(false);
	let gradientCustomTextColor: string | null = $state(null);

	// Context menu state
	let ctxMenu: { x: number; y: number; watched: Watched; contentId: number; contentType: SupportedMedia; mediaName: string } | undefined = $state(undefined);

	function getItemContentInfo(item: any): { contentId: number; contentType: SupportedMedia; mediaName: string } | undefined {
		const w = item.watched || item;
		if (w.content) {
			return {
				contentId: w.content.tmdbId,
				contentType: w.content.type as SupportedMedia,
				mediaName: w.content.title || "Unknown",
			};
		}
		if (w.game) {
			return {
				contentId: w.game.igdbId,
				contentType: "game",
				mediaName: w.game.name || "Unknown",
			};
		}
		if (w.manga) {
			return {
				contentId: w.manga.malId,
				contentType: "manga",
				mediaName: w.manga.title || "Unknown",
			};
		}
		return undefined;
	}

	function getItemWatched(item: any): Watched | undefined {
		if (item.watched) return item.watched;
		if (item.id && item.status !== undefined) return item as Watched;
		return undefined;
	}

	function handleTierItemContextMenu(e: MouseEvent, item: any) {
		e.preventDefault();
		const w = getItemWatched(item);
		const info = getItemContentInfo(item);
		if (!w || !info) return;
		ctxMenu = { x: e.clientX, y: e.clientY, watched: w, ...info };
	}

	function handleCtxMenuWatchedUpdate(w: Watched | undefined) {
		// Refresh the tierlist to reflect any changes
		if (w) {
			// Update watched data in the allWatchedMap
			const id = w.id;
			for (const tier of tiers) {
				if (tier.tierItems) {
					for (let i = 0; i < tier.tierItems.length; i++) {
						const item = tier.tierItems[i];
						if (item.watched && item.watched.id === id) {
							tier.tierItems[i] = { ...item, watched: w };
						}
					}
				}
			}
			tiers = [...tiers];
			untieredWatched = untieredWatched.map((uw) => {
				if ((uw.id === id) || (uw.watched?.id === id)) {
					return uw.watched ? { ...uw, watched: w } : w;
				}
				return uw;
			});
		}
		ctxMenu = undefined;
	}

	// Gradient presets: arrays of HSL color stops
	const gradientPresets: { name: string; id: string; stops: [number, number, number][] }[] = [
		{
			name: "Classic (Red → Green)",
			id: "classic",
			stops: [
				[0, 100, 75],     // red
				[30, 100, 75],    // orange
				[50, 100, 75],    // yellow
				[120, 100, 75],   // green
			],
		},
		{
			name: "Sunset",
			id: "sunset",
			stops: [
				[350, 85, 60],    // deep rose
				[15, 90, 62],     // warm red-orange
				[30, 95, 65],     // orange
				[42, 95, 68],     // amber
				[50, 90, 72],     // golden yellow
			],
		},
		{
			name: "Ocean",
			id: "ocean",
			stops: [
				[240, 80, 55],    // deep blue
				[210, 85, 60],    // blue
				[190, 90, 65],    // teal
				[175, 80, 70],    // cyan
			],
		},
		{
			name: "Neon",
			id: "neon",
			stops: [
				[320, 100, 65],   // neon pink
				[270, 100, 70],   // purple
				[210, 100, 65],   // blue
				[180, 100, 65],   // cyan
			],
		},
		{
			name: "Pastel Rainbow",
			id: "pastel",
			stops: [
				[0, 70, 82],      // soft red
				[40, 70, 82],     // soft orange
				[60, 65, 82],     // soft yellow
				[120, 50, 80],    // soft green
				[210, 60, 82],    // soft blue
				[270, 55, 82],    // soft purple
			],
		},
		{
			name: "Forest",
			id: "forest",
			stops: [
				[90, 50, 45],     // olive
				[110, 45, 50],    // forest green
				[140, 40, 55],    // sage
				[160, 35, 60],    // seafoam
			],
		},
		{
			name: "Cherry Blossom",
			id: "cherry",
			stops: [
				[340, 70, 55],    // deep pink
				[345, 65, 65],    // rose
				[350, 60, 75],    // light pink
				[355, 55, 82],    // blush
			],
		},
		{
			name: "Aurora",
			id: "aurora",
			stops: [
				[280, 80, 55],    // violet
				[240, 75, 60],    // blue
				[180, 85, 45],    // teal
				[140, 80, 55],    // emerald
				[60, 90, 65],     // yellow-green
			],
		},
		{
			name: "Fire",
			id: "fire",
			stops: [
				[55, 100, 75],    // bright yellow
				[35, 100, 60],    // orange
				[15, 95, 50],     // red-orange
				[0, 90, 40],      // deep red
			],
		},
		{
			name: "Lavender Dream",
			id: "lavender",
			stops: [
				[260, 65, 75],    // lavender
				[280, 55, 70],    // purple
				[300, 50, 75],    // orchid
				[320, 55, 78],    // soft pink
			],
		},
		{
			name: "Earth Tones",
			id: "earth",
			stops: [
				[25, 60, 45],     // brown
				[30, 55, 55],     // tan
				[40, 50, 65],     // sand
				[45, 45, 75],     // cream
			],
		},
		{
			name: "Cyberpunk",
			id: "cyberpunk",
			stops: [
				[300, 100, 50],   // magenta
				[270, 100, 55],   // electric purple
				[200, 100, 50],   // electric blue
				[170, 100, 50],   // neon teal
			],
		},
		{
			name: "Candy",
			id: "candy",
			stops: [
				[340, 80, 70],    // hot pink
				[20, 90, 75],     // peach
				[50, 85, 75],     // yellow
				[150, 70, 70],    // mint
				[190, 80, 72],    // sky blue
			],
		},
		{
			name: "Vintage",
			id: "vintage",
			stops: [
				[10, 45, 50],     // rust
				[25, 40, 58],     // terracotta
				[40, 35, 66],     // camel
				[55, 30, 72],     // wheat
			],
		},
		{
			name: "Royal",
			id: "royal",
			stops: [
				[45, 90, 65],     // gold
				[280, 60, 45],    // royal purple
				[220, 65, 40],    // navy
				[0, 0, 25],       // near black
			],
		},
		{
			name: "Tropical",
			id: "tropical",
			stops: [
				[50, 95, 60],     // mango
				[30, 100, 65],    // tangerine
				[350, 85, 60],    // hibiscus
				[310, 70, 55],    // orchid purple
				[170, 75, 45],    // palm green
			],
		},
		{
			name: "Ice",
			id: "ice",
			stops: [
				[200, 40, 85],    // pale blue
				[210, 55, 75],    // light steel
				[215, 65, 65],    // ice blue
				[220, 50, 55],    // steel
			],
		},
		{
			name: "Autumn",
			id: "autumn",
			stops: [
				[5, 75, 45],      // dark red
				[20, 80, 50],     // burnt orange
				[35, 85, 60],     // orange
				[45, 80, 65],     // amber
				[55, 70, 70],     // golden
			],
		},
		{
			name: "Galaxy",
			id: "galaxy",
			stops: [
				[260, 80, 25],    // deep space
				[280, 70, 40],    // nebula purple
				[240, 75, 55],    // bright blue
				[200, 80, 70],    // star cyan
				[50, 90, 80],     // starlight
			],
		},
		{
			name: "Bubblegum",
			id: "bubblegum",
			stops: [
				[330, 80, 75],    // pink
				[300, 65, 78],    // light purple
				[270, 70, 78],    // periwinkle
				[240, 75, 80],    // light blue
			],
		},
		{
			name: "Midnight",
			id: "midnight",
			stops: [
				[240, 60, 20],    // deep navy
				[250, 55, 30],    // dark indigo
				[260, 50, 40],    // twilight
				[270, 45, 50],    // dusk purple
			],
		},
		{
			name: "Rainbow",
			id: "rainbow",
			stops: [
				[0, 85, 65],      // red
				[30, 90, 65],     // orange
				[55, 90, 68],     // yellow
				[120, 70, 58],    // green
				[210, 80, 60],    // blue
				[270, 70, 60],    // indigo
				[310, 75, 65],    // violet
			],
		},
		{
			name: "Monochrome",
			id: "mono",
			stops: [
				[0, 0, 85],       // light gray
				[0, 0, 55],       // mid gray
				[0, 0, 35],       // dark gray
			],
		},
	];

	function interpolateHSL(
		stops: [number, number, number][],
		count: number,
	): { color: string; textColor: string }[] {
		if (count <= 0) return [];
		if (count === 1) {
			const [h, s, l] = stops[0];
			return [{ color: hslToHex(h, s, l), textColor: l > 65 ? "#000000" : "#ffffff" }];
		}
		const results: { color: string; textColor: string }[] = [];
		for (let i = 0; i < count; i++) {
			const t = i / (count - 1);
			const segmentCount = stops.length - 1;
			const segment = Math.min(Math.floor(t * segmentCount), segmentCount - 1);
			const localT = (t * segmentCount) - segment;
			const [h1, s1, l1] = stops[segment];
			const [h2, s2, l2] = stops[segment + 1];
			// Shortest path hue interpolation (handles wrapping around 360°)
			let dh = h2 - h1;
			if (dh > 180) dh -= 360;
			if (dh < -180) dh += 360;
			const h = ((h1 + dh * localT) % 360 + 360) % 360;
			const s = s1 + (s2 - s1) * localT;
			const l = l1 + (l2 - l1) * localT;
			results.push({
				color: hslToHex(h, s, l),
				textColor: l > 65 ? "#000000" : "#ffffff",
			});
		}
		return results;
	}

	function hslToHex(h: number, s: number, l: number): string {
		s /= 100;
		l /= 100;
		const a = s * Math.min(l, 1 - l);
		const f = (n: number) => {
			const k = (n + h / 30) % 12;
			const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1);
			return Math.round(255 * color).toString(16).padStart(2, "0");
		};
		return `#${f(0)}${f(8)}${f(4)}`;
	}

	function getGradientPreview(gradientId: string) {
		const preset = gradientPresets.find((g) => g.id === gradientId);
		if (!preset) return [];
		const stops = gradientReversed ? [...preset.stops].reverse() : preset.stops;
		const results = interpolateHSL(stops, tiers.length);
		if (gradientCustomTextColor) {
			return results.map((r) => ({ ...r, textColor: gradientCustomTextColor! }));
		}
		return results;
	}

	function applyGradient() {
		if (!selectedGradient) return;
		const colors = getGradientPreview(selectedGradient);
		tiers = tiers.map((tier, i) => ({
			...tier,
			color: colors[i]?.color ?? tier.color,
			textColor: colors[i]?.textColor ?? tier.textColor,
		}));
		showGradientModal = false;
		selectedGradient = null;
		gradientReversed = false;
		gradientCustomTextColor = null;
	}

	// Overlay state
	let overlayExpanded = $state(true);

	// Status filters for untiered items
	let activeStatusFilters: WatchedStatus[] = $state([]);

	const statusLabels: { value: WatchedStatus; label: string }[] = [
		{ value: "PLANNED", label: "Planned" },
		{ value: "WATCHING", label: "Watching" },
		{ value: "FINISHED", label: "Finished" },
		{ value: "HOLD", label: "On Hold" },
		{ value: "DROPPED", label: "Dropped" },
	];

	// Content type filter for untiered items
	type UntieredTypeFilter = "all" | "movie" | "show" | "anime" | "game" | "manga";
	let activeTypeFilter: UntieredTypeFilter = $state("all");

	const typeFilterLabels: { value: UntieredTypeFilter; label: string }[] = [
		{ value: "all", label: "All" },
		{ value: "movie", label: "Movies" },
		{ value: "show", label: "Shows" },
		{ value: "anime", label: "Anime" },
		{ value: "game", label: "Games" },
		{ value: "manga", label: "Manga" },
	];

	function getUntieredItemType(w: any): UntieredTypeFilter {
		if (w.content) {
			if (w.content.type === "tv" && w.content.isAnime) return "anime";
			return w.content.type === "movie" ? "movie" : "show";
		}
		if (w.game) return "game";
		if (w.manga) return "manga";
		return "movie";
	}

	let filteredUntiered = $derived.by(() => {
		let items = untieredWatched;
		if (activeTypeFilter !== "all") {
			items = items.filter((w: any) => getUntieredItemType(w) === activeTypeFilter);
		}
		if (activeStatusFilters.length > 0) {
			items = items.filter((w: any) => activeStatusFilters.includes(w.status));
		}
		return items;
	});

	async function exportTierlist() {
		if (!tierlistContainer || exporting) return;
		exporting = true;
		try {
			const username = store.userInfo?.username || "User";
			const tokenData = parseTokenPayload();
			const now = new Date();
			const dateStr = now.toLocaleDateString("en-US", {
				year: "numeric",
				month: "long",
				day: "numeric",
			});

			// Build share link
			let shareLink = "";
			if (tokenData) {
				shareLink = `${window.location.origin}/lists/${tokenData.userId}/${username}`;
			}

			// Create a wrapper with title for export
			const wrapper = document.createElement("div");
			wrapper.style.cssText = `
				background: #1a1a1a;
				padding: 32px;
				display: inline-block;
			`;

			// Header with title and date
			const header = document.createElement("div");
			header.style.cssText = `
				display: flex;
				justify-content: space-between;
				align-items: baseline;
				margin-bottom: 20px;
			`;

			const title = document.createElement("div");
			title.style.cssText = `
				color: #ffffff;
				font-family: system-ui, -apple-system, sans-serif;
				font-size: 28px;
				font-weight: 700;
				letter-spacing: -0.3px;
			`;
			title.textContent = `${activeTierlist?.name || 'Tierlist'} from ${username}`;

			const date = document.createElement("div");
			date.style.cssText = `
				color: rgba(255, 255, 255, 0.5);
				font-family: system-ui, -apple-system, sans-serif;
				font-size: 14px;
			`;
			date.textContent = dateStr;

			header.appendChild(title);
			header.appendChild(date);

			const clone = tierlistContainer.cloneNode(true) as HTMLElement;
			// Remove edit-only elements from clone
			clone.querySelectorAll(".tier-move-actions, .tier-label-actions").forEach((el) => el.remove());

			// Copy computed styles for tier labels
			const origLabels = tierlistContainer.querySelectorAll(".tier-label");
			const cloneLabels = clone.querySelectorAll(".tier-label");
			origLabels.forEach((orig, i) => {
				const cl = cloneLabels[i] as HTMLElement;
				if (cl) {
					const cs = getComputedStyle(orig);
					cl.style.minWidth = cs.minWidth;
				}
			});

			wrapper.appendChild(header);
			wrapper.appendChild(clone);

			// Footer with share link
			if (shareLink) {
				const footer = document.createElement("div");
				footer.style.cssText = `
					color: rgba(255, 255, 255, 0.4);
					font-family: system-ui, -apple-system, sans-serif;
					font-size: 13px;
					margin-top: 16px;
					text-align: right;
				`;
				footer.textContent = shareLink;
				wrapper.appendChild(footer);
			}

			document.body.appendChild(wrapper);

			const dataUrl = await toPng(wrapper, {
				pixelRatio: 3,
				cacheBust: true,
			});

			document.body.removeChild(wrapper);

			const link = document.createElement("a");
			link.download = `tierlist-${username}.png`;
			link.href = dataUrl;
			link.click();

			notify({ text: "Tierlist exported!", type: "success" });
		} catch (err) {
			console.error("Failed to export tierlist:", err);
			notify({ text: "Failed to export tierlist", type: "error" });
		} finally {
			exporting = false;
		}
	}

	function toggleStatusFilter(status: WatchedStatus) {
		if (activeStatusFilters.includes(status)) {
			activeStatusFilters = activeStatusFilters.filter((s) => s !== status);
		} else {
			activeStatusFilters = [...activeStatusFilters, status];
		}
	}

	// Preset definitions
	const presets = [
		{
			name: "Classic Tierlist",
			desc: "S / A / B / C / D / F",
			tiers: [
				{ name: "S", color: "#ff7f7f", textColor: "#000000" },
				{ name: "A", color: "#ffbf7f", textColor: "#000000" },
				{ name: "B", color: "#ffdf7f", textColor: "#000000" },
				{ name: "C", color: "#ffff7f", textColor: "#000000" },
				{ name: "D", color: "#bfff7f", textColor: "#000000" },
				{ name: "F", color: "#7fff7f", textColor: "#000000" },
			],
		},
		{
			name: "CS2 Rarity",
			desc: "★ / Covert / Classified / Restricted / Mil-Spec / Industrial / Consumer",
			tiers: [
				{ name: "★", color: "#ffd600", textColor: "#000000" },
				{ name: "Covert", color: "#f04b49", textColor: "#ffffff" },
				{ name: "Classified", color: "#d929ec", textColor: "#ffffff" },
				{ name: "Restricted", color: "#8c45ff", textColor: "#ffffff" },
				{ name: "Mil-Spec", color: "#4c69ff", textColor: "#ffffff" },
				{ name: "Industrial", color: "#6298d3", textColor: "#ffffff" },
				{ name: "Consumer", color: "#b4c2d8", textColor: "#000000" },
			],
		},
		{
			name: "School Grades",
			desc: "A+ / A / B / C / D / F",
			tiers: [
				{ name: "A+", color: "#4CAF50", textColor: "#ffffff" },
				{ name: "A", color: "#66BB6A", textColor: "#ffffff" },
				{ name: "B", color: "#FFC107", textColor: "#000000" },
				{ name: "C", color: "#FF9800", textColor: "#000000" },
				{ name: "D", color: "#F44336", textColor: "#ffffff" },
				{ name: "F", color: "#B71C1C", textColor: "#ffffff" },
			],
		},
		{
			name: "Olympic Medals",
			desc: "Gold / Silver / Bronze",
			tiers: [
				{ name: "🥇", color: "#FFD700", textColor: "#000000" },
				{ name: "🥈", color: "#C0C0C0", textColor: "#000000" },
				{ name: "🥉", color: "#CD7F32", textColor: "#ffffff" },
			],
		},
		{
			name: "Spicy Scale",
			desc: "🔥🔥🔥 / 🔥🔥 / 🔥 / 🧊 / 💤",
			tiers: [
				{ name: "🔥🔥🔥", color: "#d32f2f", textColor: "#ffffff" },
				{ name: "🔥🔥", color: "#f44336", textColor: "#ffffff" },
				{ name: "🔥", color: "#ff7043", textColor: "#000000" },
				{ name: "🧊", color: "#4fc3f7", textColor: "#000000" },
				{ name: "💤", color: "#455a64", textColor: "#ffffff" },
			],
		},
	];

	async function applyPreset(preset: (typeof presets)[0]) {
		const nid = notify({ text: "Applying preset...", type: "loading" });
		try {
			// Update existing tiers (reuse them, don't delete)
			for (let i = 0; i < preset.tiers.length; i++) {
				const p = preset.tiers[i];
				if (i < tiers.length) {
					// Update existing tier
					await axios.put(`/tierlist/tier/${tiers[i].id}`, {
						name: p.name,
						color: p.color,
						textColor: p.textColor,
					});
					tiers[i] = { ...tiers[i], name: p.name, color: p.color, textColor: p.textColor };
				} else {
					// Create new tier at the bottom
					const resp = await axios.post("/tierlist/tier", {
						name: p.name,
						color: p.color,
						textColor: p.textColor,
					});
					tiers = [...tiers, { ...resp.data, tierItems: [] }];
				}
			}
			tiers = [...tiers];
			showPresetModal = false;
			notify({ id: nid, text: "Preset applied!", type: "success" });
		} catch (err) {
			console.error("Failed to apply preset:", err);
			notify({ id: nid, text: "Failed to apply preset", type: "error" });
		}
	}

	// New tier form
	let newTierName = $state("");
	let newTierColor = $state("#FF7F7F");
	let newTierTextColor = $state("#000000");

	onMount(async () => {
		await loadTierlists();
		await loadUserPresets();
	});

	// Equalize all tier label widths to the widest one
	$effect(() => {
		// Re-run when tiers change or container is mounted
		void tiers;
		void editMode;
		if (!tierlistContainer) return;
		const labels = tierlistContainer.querySelectorAll<HTMLElement>(".tier-label");
		if (labels.length === 0) return;
		// Reset widths first to measure natural size
		labels.forEach((l) => (l.style.minWidth = ""));
		// Use requestAnimationFrame to measure after layout
		requestAnimationFrame(() => {
			let maxW = 100;
			labels.forEach((l) => {
				if (l.offsetWidth > maxW) maxW = l.offsetWidth;
			});
			labels.forEach((l) => (l.style.minWidth = `${maxW}px`));
		});
	});

	async function loadUserPresets() {
		try {
			const resp = await axios.get("/tierlist/presets");
			userPresets = resp.data || [];
		} catch (err) {
			console.error("Failed to load user presets:", err);
		}
	}

	async function saveCurrentAsPreset() {
		if (!savePresetName.trim()) return;
		const nid = notify({ text: "Saving preset...", type: "loading" });
		try {
			const presetTiers = tiers.map((t) => ({
				name: t.name,
				color: t.color,
				textColor: t.textColor,
			}));
			await axios.post("/tierlist/preset", {
				name: savePresetName.trim(),
				description: savePresetDesc.trim(),
				tiers: presetTiers,
			});
			await loadUserPresets();
			showSavePresetModal = false;
			savePresetName = "";
			savePresetDesc = "";
			notify({ id: nid, text: "Preset saved!", type: "success" });
		} catch (err) {
			console.error("Failed to save preset:", err);
			notify({ id: nid, text: "Failed to save preset", type: "error" });
		}
	}

	async function deleteUserPreset(presetId: number) {
		const nid = notify({ text: "Deleting preset...", type: "loading" });
		try {
			await axios.delete(`/tierlist/preset/${presetId}`);
			userPresets = userPresets.filter((p) => p.id !== presetId);
			notify({ id: nid, text: "Preset deleted!", type: "success" });
		} catch (err) {
			console.error("Failed to delete preset:", err);
			notify({ id: nid, text: "Failed to delete preset", type: "error" });
		}
	}

	function applyUserPreset(preset: TierPreset) {
		applyPreset({
			name: preset.name,
			desc: preset.description,
			tiers: preset.tiers.map((t) => ({
				name: t.name,
				color: t.color,
				textColor: t.textColor,
			})),
		});
	}

	async function loadTierlists() {
		loading = true;
		try {
			const resp = await axios.get("/tierlist/lists");
			tierlists = resp.data || [];
			if (tierlists.length === 0) {
				// Create a default tierlist
				const createResp = await axios.post("/tierlist/list", { name: "Tierlist" });
				tierlists = [createResp.data];
			}
			// Select first tierlist if none active
			if (!activeTierlistId || !tierlists.find((tl) => tl.id === activeTierlistId)) {
				activeTierlistId = tierlists[0].id;
			}
			await loadTierlist();
		} catch (err) {
			console.error("Failed to load tierlists:", err);
			notify({ text: "Failed to load tierlists", type: "error" });
			loading = false;
		}
	}

	async function loadTierlist() {
		loading = true;
		try {
			const [tiersResp, untieredResp] = await Promise.all([
				axios.get(`/tierlist?listId=${activeTierlistId}`),
				axios.get(`/tierlist/untiered?listId=${activeTierlistId}`),
			]);
			tiers = tiersResp.data || [];
			untieredWatched = untieredResp.data || [];

			// If no tiers exist, create defaults
			if (tiers.length === 0) {
				const defaultResp = await axios.post(`/tierlist/defaults?listId=${activeTierlistId}`);
				tiers = defaultResp.data || [];
			}
		} catch (err) {
			console.error("Failed to load tierlist:", err);
			notify({ text: "Failed to load tierlist", type: "error" });
		}
		loading = false;
	}

	async function switchTierlist(id: number) {
		if (id === activeTierlistId) return;
		activeTierlistId = id;
		editMode = false;
		await loadTierlist();
	}

	async function createTierlist() {
		if (!newTierlistName.trim()) return;
		const nid = notify({ text: "Creating tierlist...", type: "loading" });
		try {
			const resp = await axios.post("/tierlist/list", { name: newTierlistName.trim() });
			tierlists = [...tierlists, resp.data];
			activeTierlistId = resp.data.id;
			showCreateTierlistModal = false;
			newTierlistName = "";
			notify({ id: nid, text: "Tierlist created!", type: "success" });
			await loadTierlist();
		} catch (err) {
			console.error("Failed to create tierlist:", err);
			notify({ id: nid, text: "Failed to create tierlist", type: "error" });
		}
	}

	async function renameTierlist() {
		if (!activeTierlistId || !renameTierlistName.trim()) return;
		const nid = notify({ text: "Renaming...", type: "loading" });
		try {
			await axios.put(`/tierlist/list/${activeTierlistId}`, { name: renameTierlistName.trim() });
			tierlists = tierlists.map((tl) =>
				tl.id === activeTierlistId ? { ...tl, name: renameTierlistName.trim() } : tl,
			);
			showRenameTierlistModal = false;
			renameTierlistName = "";
			notify({ id: nid, text: "Tierlist renamed!", type: "success" });
		} catch (err) {
			console.error("Failed to rename tierlist:", err);
			notify({ id: nid, text: "Failed to rename tierlist", type: "error" });
		}
	}

	async function deleteTierlist() {
		if (!activeTierlistId || tierlists.length <= 1) return;
		const nid = notify({ text: "Deleting tierlist...", type: "loading" });
		try {
			await axios.delete(`/tierlist/list/${activeTierlistId}`);
			tierlists = tierlists.filter((tl) => tl.id !== activeTierlistId);
			activeTierlistId = tierlists[0]?.id ?? null;
			notify({ id: nid, text: "Tierlist deleted!", type: "success" });
			if (activeTierlistId) await loadTierlist();
		} catch (err) {
			console.error("Failed to delete tierlist:", err);
			notify({ id: nid, text: "Failed to delete tierlist", type: "error" });
		}
	}

	function getItemPoster(item: any): string | undefined {
		// item can be a TierItem (with .watched) or a direct Watched object
		const w = item.watched || item;
		if (w.content?.poster_path) {
			return `${baseURL}/img${w.content.poster_path}`;
		}
		if (w.game?.poster?.path) {
			return `${baseURL}/${w.game.poster.path}`;
		}
		if (w.game?.coverId) {
			return `https://images.igdb.com/igdb/image/upload/t_cover_big/${w.game.coverId}.jpg`;
		}
		if (w.manga?.poster?.path) {
			return `${baseURL}/${w.manga.poster.path}`;
		}
		if (w.manga?.posterUrl) {
			return w.manga.posterUrl;
		}
		return undefined;
	}

	function getItemTitle(item: any): string {
		const w = item.watched || item;
		if (w.content?.title) return w.content.title;
		if (w.game?.name) return w.game.name;
		if (w.manga?.title) return w.manga.title;
		return "Unknown";
	}

	function getItemLink(item: any): string | undefined {
		const w = item.watched || item;
		if (w.content) {
			return `/${w.content.type}/${w.content.tmdbId}`;
		}
		if (w.game) {
			return `/game/${w.game.igdbId}`;
		}
		if (w.manga) {
			return `/manga/${w.manga.malId}`;
		}
		return undefined;
	}

	function getWatchedId(item: any): number {
		if (item.watchedId) return item.watchedId;
		if (item.watched?.id) return item.watched.id;
		return item.id;
	}

	// Drag and Drop
	function onDragStart(
		e: DragEvent,
		watchedId: number,
		sourceTierId: number | null,
		sourceIndex: number,
	) {
		if (!editMode) return;
		dragItem = { watchedId, sourceTierId, sourceIndex };
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = "move";
			e.dataTransfer.setData("text/plain", String(watchedId));
		}
	}

	function onDragOverContainer(e: DragEvent, tierId: number, items: any[]) {
		if (!editMode || !dragItem) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = "move";

		// Calculate drop index from mouse position on a virtual grid.
		// This avoids the flickering feedback loop caused by DOM placeholder
		// shifting items under the cursor.
		const container = e.currentTarget as HTMLElement;
		const rect = container.getBoundingClientRect();
		const pad = 8;
		const gap = 6;
		const itemW = 68;
		const slotW = itemW + gap;
		const itemH = 120;

		const mouseX = e.clientX - rect.left - pad;
		const mouseY = e.clientY - rect.top - pad;
		const innerW = rect.width - pad * 2;
		const cols = Math.max(1, Math.floor((innerW + gap) / slotW));
		const col = Math.max(0, Math.floor((mouseX + gap / 2) / slotW));
		const row = Math.max(0, Math.floor((mouseY + gap / 2) / itemH));

		// Check if placeholder is currently rendered (it takes a visual slot)
		const phShown =
			dragOverTierId === tierId &&
			dragOverIndex !== null &&
			(dragOverIndex >= items.length ||
				dragItem.watchedId !== getWatchedId(items[dragOverIndex]));

		const visualSlots = items.length + (phShown ? 1 : 0);
		let visualIdx = Math.min(row * cols + col, visualSlots);

		// Convert visual index → data index, compensating for placeholder slot
		let dataIdx: number;
		if (phShown && dragOverIndex !== null) {
			dataIdx = visualIdx <= dragOverIndex ? visualIdx : visualIdx - 1;
		} else {
			dataIdx = visualIdx;
		}
		dataIdx = Math.max(0, Math.min(dataIdx, items.length));

		dragOverTierId = tierId;
		dragOverIndex = dataIdx;
	}

	function onDrop(e: DragEvent, targetTierId: number) {
		if (!editMode || !dragItem || dragOverIndex === null) return;
		e.preventDefault();

		const { watchedId, sourceTierId, sourceIndex } = dragItem;
		let insertIndex = dragOverIndex;

		// Adjust index for same-tier: removal shifts items after source position
		if (sourceTierId === targetTierId && sourceIndex < insertIndex) {
			insertIndex--;
		}

		// Capture watched data BEFORE removing from source
		let watchedData: any;
		if (sourceTierId === null) {
			watchedData = untieredWatched.find((w) => getWatchedId(w) === watchedId);
		} else {
			const sourceTier = tiers.find((t) => t.id === sourceTierId);
			const sourceItem = sourceTier?.tierItems?.find(
				(item) => getWatchedId(item) === watchedId,
			);
			watchedData = sourceItem?.watched || sourceItem;
		}

		// Remove from source
		if (sourceTierId === null) {
			untieredWatched = untieredWatched.filter((w) => getWatchedId(w) !== watchedId);
		} else {
			const sourceTier = tiers.find((t) => t.id === sourceTierId);
			if (sourceTier?.tierItems) {
				sourceTier.tierItems = sourceTier.tierItems.filter(
					(item) => getWatchedId(item) !== watchedId,
				);
			}
		}

		const itemToInsert = {
			watchedId,
			tierId: targetTierId,
			position: insertIndex,
			watched: watchedData,
		};

		const targetTier = tiers.find((t) => t.id === targetTierId);
		if (targetTier) {
			if (!targetTier.tierItems) targetTier.tierItems = [];
			targetTier.tierItems.splice(insertIndex, 0, itemToInsert);
			targetTier.tierItems = targetTier.tierItems.map((item, i) => ({
				...item,
				position: i,
			}));
		}

		tiers = [...tiers];
		dragItem = null;
		dragOverTierId = null;
		dragOverIndex = null;
	}

	function onDropToUntiered(e: DragEvent) {
		if (!editMode || !dragItem) return;
		e.preventDefault();

		const { watchedId, sourceTierId } = dragItem;

		if (sourceTierId !== null) {
			// Remove from tier
			const sourceTier = tiers.find((t) => t.id === sourceTierId);
			if (sourceTier?.tierItems) {
				const removed = sourceTier.tierItems.find(
					(item) => getWatchedId(item) === watchedId,
				);
				sourceTier.tierItems = sourceTier.tierItems.filter(
					(item) => getWatchedId(item) !== watchedId,
				);
				if (removed) {
					const watchedData = removed.watched || findWatchedData(watchedId);
					if (watchedData) {
						untieredWatched = [...untieredWatched, watchedData];
					}
				}
			}
			tiers = [...tiers];
		}

		dragItem = null;
		dragOverTierId = null;
		dragOverIndex = null;
	}

	function onDragEnd() {
		dragItem = null;
		dragOverTierId = null;
		dragOverIndex = null;
	}

	// === Touch drag support for mobile ===
	let touchDragEl: HTMLElement | null = null;
	let touchOffsetX = 0;
	let touchOffsetY = 0;
	let touchOverUntiered = false;

	function onTouchDragStart(
		e: TouchEvent,
		watchedId: number,
		sourceTierId: number | null,
		sourceIndex: number,
	) {
		if (!editMode) return;
		const touch = e.touches[0];
		const target = e.currentTarget as HTMLElement;

		dragItem = { watchedId, sourceTierId, sourceIndex };
		touchOverUntiered = false;

		// Create visual clone
		const clone = target.cloneNode(true) as HTMLElement;
		clone.style.cssText = `
			position: fixed;
			z-index: 9999;
			pointer-events: none;
			opacity: 0.85;
			width: ${target.offsetWidth}px;
			transform: scale(1.08);
		`;

		const rect = target.getBoundingClientRect();
		touchOffsetX = touch.clientX - rect.left;
		touchOffsetY = touch.clientY - rect.top;
		clone.style.left = (touch.clientX - touchOffsetX) + 'px';
		clone.style.top = (touch.clientY - touchOffsetY) + 'px';

		document.body.appendChild(clone);
		touchDragEl = clone;

		document.addEventListener('touchmove', handleTouchMove, { passive: false });
		document.addEventListener('touchend', handleTouchEnd);
		document.addEventListener('touchcancel', handleTouchEnd);

		e.preventDefault();
	}

	function handleTouchMove(e: TouchEvent) {
		if (!dragItem || !touchDragEl) return;
		e.preventDefault();

		const touch = e.touches[0];
		touchDragEl.style.left = (touch.clientX - touchOffsetX) + 'px';
		touchDragEl.style.top = (touch.clientY - touchOffsetY) + 'px';

		// Find element under finger (hide clone temporarily)
		touchDragEl.style.display = 'none';
		const elUnder = document.elementFromPoint(touch.clientX, touch.clientY);
		touchDragEl.style.display = '';

		if (!elUnder) return;

		const tierContainer = elUnder.closest('[data-tier-id]') as HTMLElement;
		const untieredEl = elUnder.closest('.untiered-items') || elUnder.closest('.untiered-section');

		if (tierContainer) {
			const tierId = parseInt(tierContainer.dataset.tierId!);
			const tierData = tiers.find((t) => t.id === tierId);
			if (tierData) {
				touchOverUntiered = false;
				const items = tierData.tierItems || [];
				const rect = tierContainer.getBoundingClientRect();
				const pad = 8;
				const gap = 6;
				const isMobile = window.innerWidth <= 600;
				const itemW = isMobile ? 50 : 68;
				const slotW = itemW + gap;
				const itemH = isMobile ? 87 : 120;

				const mx = touch.clientX - rect.left - pad;
				const my = touch.clientY - rect.top - pad;
				const innerW = rect.width - pad * 2;
				const cols = Math.max(1, Math.floor((innerW + gap) / slotW));
				const col = Math.max(0, Math.floor((mx + gap / 2) / slotW));
				const row = Math.max(0, Math.floor((my + gap / 2) / itemH));

				let dataIdx = Math.min(row * cols + col, items.length);
				dataIdx = Math.max(0, dataIdx);

				dragOverTierId = tierId;
				dragOverIndex = dataIdx;
			}
		} else if (untieredEl) {
			touchOverUntiered = true;
			dragOverTierId = null;
			dragOverIndex = null;
			if (!overlayExpanded) overlayExpanded = true;
		} else {
			touchOverUntiered = false;
			dragOverTierId = null;
			dragOverIndex = null;
		}
	}

	function handleTouchEnd() {
		document.removeEventListener('touchmove', handleTouchMove);
		document.removeEventListener('touchend', handleTouchEnd);
		document.removeEventListener('touchcancel', handleTouchEnd);

		if (dragItem) {
			if (dragOverTierId !== null && dragOverIndex !== null) {
				// Drop into a tier
				const { watchedId, sourceTierId, sourceIndex } = dragItem;
				let insertIndex = dragOverIndex;

				if (sourceTierId === dragOverTierId && sourceIndex < insertIndex) {
					insertIndex--;
				}

				let watchedData: any;
				if (sourceTierId === null) {
					watchedData = untieredWatched.find((w) => getWatchedId(w) === watchedId);
				} else {
					const srcTier = tiers.find((t) => t.id === sourceTierId);
					const srcItem = srcTier?.tierItems?.find(
						(item) => getWatchedId(item) === watchedId,
					);
					watchedData = srcItem?.watched || srcItem;
				}

				if (sourceTierId === null) {
					untieredWatched = untieredWatched.filter(
						(w) => getWatchedId(w) !== watchedId,
					);
				} else {
					const srcTier = tiers.find((t) => t.id === sourceTierId);
					if (srcTier?.tierItems) {
						srcTier.tierItems = srcTier.tierItems.filter(
							(item) => getWatchedId(item) !== watchedId,
						);
					}
				}

				const targetTier = tiers.find((t) => t.id === dragOverTierId);
				if (targetTier) {
					if (!targetTier.tierItems) targetTier.tierItems = [];
					targetTier.tierItems.splice(insertIndex, 0, {
						watchedId,
						tierId: dragOverTierId!,
						position: insertIndex,
						watched: watchedData,
					});
					targetTier.tierItems = targetTier.tierItems.map((item, i) => ({
						...item,
						position: i,
					}));
				}

				tiers = [...tiers];
			} else if (touchOverUntiered && dragItem.sourceTierId !== null) {
				// Drop to untiered
				const { watchedId, sourceTierId } = dragItem;
				const srcTier = tiers.find((t) => t.id === sourceTierId);
				if (srcTier?.tierItems) {
					const removed = srcTier.tierItems.find(
						(item) => getWatchedId(item) === watchedId,
					);
					srcTier.tierItems = srcTier.tierItems.filter(
						(item) => getWatchedId(item) !== watchedId,
					);
					if (removed) {
						const wd = removed.watched || findWatchedData(watchedId);
						if (wd) untieredWatched = [...untieredWatched, wd];
					}
				}
				tiers = [...tiers];
			}
		}

		// Cleanup
		if (touchDragEl) {
			touchDragEl.remove();
			touchDragEl = null;
		}
		dragItem = null;
		dragOverTierId = null;
		dragOverIndex = null;
		touchOverUntiered = false;
	}

	// Store all watched data for lookup during drag/drop
	let allWatchedMap: Map<number, any> = $derived.by(() => {
		const map = new Map();
		for (const tier of tiers) {
			if (tier.tierItems) {
				for (const item of tier.tierItems) {
					if (item.watched) {
						map.set(getWatchedId(item), item.watched);
					} else {
						map.set(getWatchedId(item), item);
					}
				}
			}
		}
		for (const w of untieredWatched) {
			map.set(getWatchedId(w), w);
		}
		return map;
	});

	function findWatchedData(watchedId: number): any {
		return allWatchedMap.get(watchedId);
	}

	async function saveChanges() {
		saving = true;
		const nid = notify({ text: "Saving tierlist...", type: "loading" });
		try {
			// Save tier colors/names
			for (const tier of tiers) {
				await axios.put(`/tierlist/tier/${tier.id}`, {
					name: tier.name,
					color: tier.color,
					textColor: tier.textColor,
				});
			}
			// Build items map
			const items: Record<number, number[]> = {};
			for (const tier of tiers) {
				items[tier.id] = (tier.tierItems || []).map((item) => getWatchedId(item));
			}
			await axios.put("/tierlist/items", { items });
			// Also sync ratings for backwards compatibility
			await axios.post(`/tierlist/sync-ratings?listId=${activeTierlistId}`);
			notify({ id: nid, text: "Tierlist saved!", type: "success" });
		} catch (err) {
			console.error("Failed to save tierlist:", err);
			notify({ id: nid, text: "Failed to save!", type: "error" });
		}
		saving = false;
	}

	async function addTier() {
		if (!newTierName.trim()) return;
		const req: CreateTierRequest = {
			name: newTierName.trim(),
			color: newTierColor,
			textColor: newTierTextColor,
		};
		try {
			const resp = await axios.post("/tierlist/tier", { ...req, tierlistId: activeTierlistId });
			tiers = [...tiers, { ...resp.data, tierItems: [] }];
			showAddTierModal = false;
			newTierName = "";
			newTierColor = "#FF7F7F";
			newTierTextColor = "#000000";
		} catch (err) {
			console.error("Failed to create tier:", err);
			notify({ text: "Failed to create tier", type: "error" });
		}
	}

	async function updateTierDetails() {
		if (!editingTier) return;
		try {
			await axios.put(`/tierlist/tier/${editingTier.id}`, {
				name: editingTier.name,
				color: editingTier.color,
				textColor: editingTier.textColor,
			});
			tiers = tiers.map((t) =>
				t.id === editingTier!.id
					? { ...t, name: editingTier!.name, color: editingTier!.color, textColor: editingTier!.textColor }
					: t,
			);
			editingTier = null;
			notify({ text: "Tier updated!", type: "success" });
		} catch (err) {
			console.error("Failed to update tier:", err);
			notify({ text: "Failed to update tier", type: "error" });
		}
	}

	async function deleteTier(tierId: number) {
		try {
			const tier = tiers.find((t) => t.id === tierId);
			// Move items back to untiered
			if (tier?.tierItems) {
				for (const item of tier.tierItems) {
					const w = item.watched || findWatchedData(getWatchedId(item));
					if (w) untieredWatched = [...untieredWatched, w];
				}
			}
			await axios.delete(`/tierlist/tier/${tierId}`);
			tiers = tiers.filter((t) => t.id !== tierId);
			notify({ text: "Tier deleted!", type: "success" });
		} catch (err) {
			console.error("Failed to delete tier:", err);
			notify({ text: "Failed to delete tier", type: "error" });
		}
	}

	async function moveTierUp(tierIndex: number) {
		if (tierIndex <= 0) return;
		const newTiers = [...tiers];
		[newTiers[tierIndex - 1], newTiers[tierIndex]] = [newTiers[tierIndex], newTiers[tierIndex - 1]];
		tiers = newTiers;
		try {
			await axios.put("/tierlist/reorder", { tierIds: tiers.map((t) => t.id) });
		} catch (err) {
			console.error("Failed to reorder tiers:", err);
			notify({ text: "Failed to reorder tiers", type: "error" });
		}
	}

	async function moveTierDown(tierIndex: number) {
		if (tierIndex >= tiers.length - 1) return;
		const newTiers = [...tiers];
		[newTiers[tierIndex], newTiers[tierIndex + 1]] = [newTiers[tierIndex + 1], newTiers[tierIndex]];
		tiers = newTiers;
		try {
			await axios.put("/tierlist/reorder", { tierIds: tiers.map((t) => t.id) });
		} catch (err) {
			console.error("Failed to reorder tiers:", err);
			notify({ text: "Failed to reorder tiers", type: "error" });
		}
	}
</script>

<svelte:head>
	<title>Tierlist</title>
</svelte:head>

<div class="tierlist-page" class:edit-mode={editMode}>
	<div class="tierlist-header">
		<div class="tierlist-selector">
			{#each tierlists as tl (tl.id)}
				<button
					class="tls-tab"
					class:active={activeTierlistId === tl.id}
					onclick={() => switchTierlist(tl.id)}
				>
					{tl.name}
				</button>
			{/each}
			<div class="tls-actions">
				<button class="tls-icon" onclick={() => { newTierlistName = ''; showCreateTierlistModal = true; }} title="New Tierlist">
					+
				</button>
				{#if activeTierlist}
					<button class="tls-icon" onclick={() => { renameTierlistName = activeTierlist.name; showRenameTierlistModal = true; }} title="Rename">
						<Icon i="pencil" wh={12} />
					</button>
					{#if tierlists.length > 1}
						<button class="tls-icon tls-icon-delete" onclick={deleteTierlist} title="Delete">
							<Icon i="trash" wh={12} />
						</button>
					{/if}
				{/if}
			</div>
		</div>
		<div class="tierlist-actions">
			{#if editMode}
				<button class="btn-save" onclick={saveChanges} disabled={saving}>
					{#if saving}Saving...{:else}Save{/if}
				</button>
				<button class="btn-add" onclick={() => (showPresetModal = true)}>
					Presets
				</button>
				<button class="btn-add" onclick={() => (showAddTierModal = true)}>
					Add Tier
				</button>
				<button class="btn-add" onclick={() => { selectedGradient = 'classic'; showGradientModal = true; }}>
					Auto Color
				</button>
			{/if}
			<button class="btn-add" onclick={exportTierlist} disabled={exporting || loading}>
				{#if exporting}Exporting...{:else}Export PNG{/if}
			</button>
			<button
				class="btn-edit"
				class:active={editMode}
				onclick={() => {
					editMode = !editMode;
				}}
			>
				<Icon i="pencil" wh={18} />
				{editMode ? "Done" : "Edit"}
			</button>
		</div>
	</div>

	{#if loading}
		<Spinner />
	{:else}
		<div class="tierlist-container" bind:this={tierlistContainer}>
			{#each tiers as tier, tierIdx (tier.id)}
				<div
					class="tier-row"
					class:drag-over={dragOverTierId === tier.id}
				>
					<div
						class="tier-label"
						style="background-color: {tier.color}; color: {tier.textColor}; --tier-name-len: {tier.name.length};"
					>
						{#if editMode}
							<div class="tier-move-actions">
								<button
									class="tier-btn"
									onclick={() => moveTierUp(tierIdx)}
									title="Move up"
									disabled={tierIdx === 0}
								>
									<Icon i="chevron" wh={12} facing="up" />
								</button>
								<button
									class="tier-btn"
									onclick={() => moveTierDown(tierIdx)}
									title="Move down"
									disabled={tierIdx === tiers.length - 1}
								>
									<Icon i="chevron" wh={12} facing="down" />
								</button>
							</div>
						{/if}
						<span class="tier-name">{tier.name}</span>
						{#if editMode}
							<div class="tier-label-actions">
								<button
									class="tier-btn"
									onclick={() => {
										editingTier = { ...tier };
									}}
									title="Edit tier"
								>
									<Icon i="pencil" wh={12} />
								</button>
								<button
									class="tier-btn danger"
									onclick={() => deleteTier(tier.id)}
									title="Delete tier"
								>
									<Icon i="trash" wh={12} />
								</button>
							</div>
						{/if}
					</div>
					<div
						class="tier-items"
						data-tier-id={tier.id}
						ondragover={(e) => onDragOverContainer(e, tier.id, tier.tierItems || [])}
						ondrop={(e) => {
							e.preventDefault();
							onDrop(e, tier.id);
						}}
					>
						{#if tier.tierItems && tier.tierItems.length > 0}
							{#each tier.tierItems as item, idx (getWatchedId(item))}
								{@const poster = getItemPoster(item)}
								{@const title = getItemTitle(item)}
								{@const link = getItemLink(item)}
								{@const showPlaceholder = dragItem && dragOverTierId === tier.id && dragOverIndex === idx && dragItem.watchedId !== getWatchedId(item) && !(dragItem.sourceTierId === tier.id && (dragOverIndex === dragItem.sourceIndex || dragOverIndex === dragItem.sourceIndex + 1))}
								{#if showPlaceholder}
									<div class="drop-placeholder"></div>
								{/if}
								<div
									class="tier-item"
									class:dragging={dragItem?.watchedId === getWatchedId(item)}
									draggable={editMode ? "true" : "false"}
									ondragstart={(e) =>
										onDragStart(e, getWatchedId(item), tier.id, idx)}
									ondragend={onDragEnd}
									ontouchstart={(e) =>
										onTouchDragStart(e, getWatchedId(item), tier.id, idx)}
									oncontextmenu={(e) => handleTierItemContextMenu(e, item)}
									title={title}
								>
									<div class="tier-item-poster">
										{#if !editMode && link}
											<a href={link}>
												{#if poster}
													<img src={poster} alt={title} loading="lazy" />
												{:else}
													<div class="no-poster">{title}</div>
												{/if}
											</a>
										{:else if poster}
											<img src={poster} alt={title} loading="lazy" />
										{:else}
											<div class="no-poster">{title}</div>
										{/if}
									</div>
									<span class="tier-item-title">{title}</span>
								</div>
							{/each}
						{/if}
						{#if dragItem && dragOverTierId === tier.id && dragOverIndex === (tier.tierItems?.length ?? 0) && !(dragItem.sourceTierId === tier.id && (dragOverIndex === dragItem.sourceIndex || dragOverIndex === dragItem.sourceIndex + 1))}
							<div class="drop-placeholder"></div>
						{/if}
						{#if editMode && (!tier.tierItems || tier.tierItems.length === 0) && !(dragItem && dragOverTierId === tier.id)}
							<div class="tier-empty-hint">Drop items here</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		{#if editMode}
			<div class="untiered-section" class:collapsed={!overlayExpanded} class:dragging={dragItem !== null}>
				<div class="untiered-header">
					<h3>Untiered ({filteredUntiered.length}/{untieredWatched.length})</h3>
					<div class="untiered-filters">
						<div class="filter-group">
							{#each typeFilterLabels as tl}
								<button
									class="filter-chip"
									class:active={activeTypeFilter === tl.value}
									onclick={() => (activeTypeFilter = tl.value)}
								>
									{tl.label}
								</button>
							{/each}
						</div>
						<span class="filter-sep">|</span>
						<div class="filter-group">
							{#each statusLabels as sl}
								<button
									class="filter-chip"
									class:active={activeStatusFilters.includes(sl.value)}
									onclick={() => toggleStatusFilter(sl.value)}
								>
									{sl.label}
								</button>
							{/each}
						</div>
					</div>
					<button class="overlay-toggle" title={overlayExpanded ? "Collapse" : "Expand"} onclick={() => (overlayExpanded = !overlayExpanded)}>
						<Icon i="chevron" wh={14} facing={overlayExpanded ? "down" : "up"} />
					</button>
				</div>
				{#if overlayExpanded}
					<div
						class="untiered-items"
						class:drag-over={dragOverTierId === null && dragItem !== null}
						ondragover={(e) => {
							e.preventDefault();
							dragOverTierId = null;
						}}
						ondrop={onDropToUntiered}
					>
						{#if filteredUntiered.length > 0}
							{#each filteredUntiered as w, idx (getWatchedId(w))}
								{@const poster = getItemPoster(w)}
								{@const title = getItemTitle(w)}
								<div
									class="tier-item"
									class:dragging={dragItem?.watchedId === getWatchedId(w)}
									draggable="true"
									ondragstart={(e) =>
										onDragStart(e, getWatchedId(w), null, idx)}
									ondragend={onDragEnd}
									ontouchstart={(e) =>
										onTouchDragStart(e, getWatchedId(w), null, idx)}
									oncontextmenu={(e) => handleTierItemContextMenu(e, w)}
									title={title}
								>
									<div class="tier-item-poster">
										{#if poster}
											<img src={poster} alt={title} loading="lazy" />
										{:else}
											<div class="no-poster">{title}</div>
										{/if}
									</div>
									<span class="tier-item-title">{title}</span>
								</div>
							{/each}
						{:else if untieredWatched.length > 0}
							<div class="untiered-empty">No items match the selected filters</div>
						{:else}
							<div class="untiered-empty">All items are in tiers!</div>
						{/if}
					</div>
				{/if}
			</div>
			{#if dragItem && !overlayExpanded}
				<div
					class="overlay-hotzone"
					ondragover={(e) => {
						e.preventDefault();
						overlayExpanded = true;
					}}
				></div>
			{/if}
		{/if}
	{/if}
</div>

<!-- Add Tier Modal -->
{#if showCreateTierlistModal}
	<Modal title="New Tierlist" onClose={() => (showCreateTierlistModal = false)}>
		<div class="tier-form">
			<label>
				Name
				<input type="text" bind:value={newTierlistName} placeholder="e.g. Anime Tierlist" />
			</label>
			<button onclick={createTierlist}>Create</button>
		</div>
	</Modal>
{/if}

{#if showRenameTierlistModal}
	<Modal title="Rename Tierlist" onClose={() => (showRenameTierlistModal = false)}>
		<div class="tier-form">
			<label>
				Name
				<input type="text" bind:value={renameTierlistName} placeholder="Tierlist name" />
			</label>
			<button onclick={renameTierlist}>Save</button>
		</div>
	</Modal>
{/if}

{#if showAddTierModal}
	<Modal title="Add Tier" onClose={() => (showAddTierModal = false)}>
		<div class="tier-form">
			<label>
				Name
				<input type="text" bind:value={newTierName} placeholder="e.g. S, A, B..." />
			</label>
			<label>
				Background Color
				<div class="color-picker-row">
					<input type="color" bind:value={newTierColor} />
					<input type="text" bind:value={newTierColor} class="color-text" />
					<div class="color-preview" style="background-color: {newTierColor}; color: {newTierTextColor};">
						{newTierName || "Preview"}
					</div>
				</div>
			</label>
			<label>
				Text Color
				<div class="color-picker-row">
					<input type="color" bind:value={newTierTextColor} />
					<input type="text" bind:value={newTierTextColor} class="color-text" />
				</div>
			</label>
			<button onclick={addTier}>Create Tier</button>
		</div>
	</Modal>
{/if}

<!-- Edit Tier Modal -->
{#if editingTier}
	<Modal title="Edit Tier" onClose={() => (editingTier = null)}>
		<div class="tier-form">
			<label>
				Name
				<input type="text" bind:value={editingTier.name} />
			</label>
			<label>
				Background Color
				<div class="color-picker-row">
					<input type="color" bind:value={editingTier.color} />
					<input type="text" bind:value={editingTier.color} class="color-text" />
					<div class="color-preview" style="background-color: {editingTier.color}; color: {editingTier.textColor};">
						{editingTier.name || "Preview"}
					</div>
				</div>
			</label>
			<label>
				Text Color
				<div class="color-picker-row">
					<input type="color" bind:value={editingTier.textColor} />
					<input type="text" bind:value={editingTier.textColor} class="color-text" />
				</div>
			</label>
			<button onclick={updateTierDetails}>Save Changes</button>
		</div>
	</Modal>
{/if}

<!-- Preset Modal -->
{#if showPresetModal}
	<Modal title="Apply Preset" onClose={() => (showPresetModal = false)}>
		<div class="preset-list">
			<div class="preset-section-label">Built-in</div>
			{#each presets as preset}
				<button class="preset-card" onclick={() => applyPreset(preset)}>
					<div class="preset-card-header">{preset.name}</div>
					<div class="preset-card-preview">
						{#each preset.tiers as t}
							<div
								class="preset-tier-chip"
								style="background-color: {t.color}; color: {t.textColor};"
							>
								{t.name}
							</div>
						{/each}
					</div>
					<div class="preset-card-desc">{preset.desc}</div>
				</button>
			{/each}

			{#if userPresets.length > 0}
				<div class="preset-section-label" style="margin-top: 12px;">User Presets</div>
				{#each userPresets as preset}
					<div class="preset-card-wrapper">
						<button class="preset-card" onclick={() => applyUserPreset(preset)}>
							<div class="preset-card-header">{preset.name}</div>
							<div class="preset-card-preview">
								{#each preset.tiers as t}
									<div
										class="preset-tier-chip"
										style="background-color: {t.color}; color: {t.textColor};"
									>
										{t.name}
									</div>
								{/each}
							</div>
							{#if preset.description}
								<div class="preset-card-desc">{preset.description}</div>
							{/if}
						</button>
						<button
							class="preset-delete-btn"
							title="Delete preset"
							onclick={(e: MouseEvent) => { e.stopPropagation(); deleteUserPreset(preset.id); }}
						>
							<Icon i="trash" wh={14} />
						</button>
					</div>
				{/each}
			{/if}
		</div>
		<button class="btn-save-preset" onclick={() => { showPresetModal = false; showSavePresetModal = true; }}>
			Save Current Tiers as Preset
		</button>
		<p class="preset-note">Existing tiers will be renamed & recolored. Extra tiers beyond the preset are kept. New tiers are added if needed.</p>
	</Modal>
{/if}

<!-- Auto Color Gradient Modal -->
{#if showGradientModal}
	<Modal title="Auto Color Gradient" onClose={() => { showGradientModal = false; selectedGradient = null; gradientReversed = false; gradientCustomTextColor = null; }}>
		<div class="gradient-modal">
			<div class="gradient-options">
				{#each gradientPresets as gp}
					<button
						class="gradient-option"
						class:selected={selectedGradient === gp.id}
						onclick={() => (selectedGradient = gp.id)}
					>
						<div class="gradient-swatch">
							{#each interpolateHSL(gradientReversed ? [...gp.stops].reverse() : gp.stops, Math.max(tiers.length, 4)) as c}
								<div style="background-color: {c.color}; flex: 1;"></div>
							{/each}
						</div>
						<span>{gp.name}</span>
					</button>
				{/each}
			</div>

			{#if selectedGradient}
				<div class="gradient-preview-section">
					<div class="gradient-preview-label">Preview</div>
					<div class="gradient-preview-tiers">
						{#each getGradientPreview(selectedGradient) as colors, i}
							<div
								class="gradient-preview-tier"
								style="background-color: {colors.color}; color: {colors.textColor};"
							>
								{tiers[i]?.name ?? ""}
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<div class="gradient-actions">
				<button class="btn-reverse" onclick={() => (gradientReversed = !gradientReversed)} title="Reverse gradient direction">
					<Icon i="sort" wh={16} />
					Reverse
				</button>
				<div class="text-color-toggle">
					<button
						class="btn-reverse"
						class:active={gradientCustomTextColor !== null}
						onclick={() => { gradientCustomTextColor = gradientCustomTextColor !== null ? null : '#ffffff'; }}
						title="Override text color for all tiers"
					>
						<Icon i="pencil" wh={14} />
						Text Color
					</button>
					{#if gradientCustomTextColor !== null}
						<input type="color" bind:value={gradientCustomTextColor} class="text-color-picker" />
					{/if}
				</div>
				<button class="btn-save" onclick={applyGradient} disabled={!selectedGradient}>
					Apply Gradient
				</button>
			</div>
		</div>
	</Modal>
{/if}

{#if showSavePresetModal}
	<Modal title="Save Preset" onClose={() => (showSavePresetModal = false)}>
		<div class="save-preset-form">
			<label>
				Name
				<input type="text" bind:value={savePresetName} placeholder="My Preset" />
			</label>
			<label>
				Description (optional)
				<input type="text" bind:value={savePresetDesc} placeholder="Short description..." />
			</label>
			<div class="save-preset-preview">
				{#each tiers as t}
					<div
						class="preset-tier-chip"
						style="background-color: {t.color}; color: {t.textColor};"
					>
						{t.name}
					</div>
				{/each}
			</div>
			<button class="btn-add" onclick={saveCurrentAsPreset} disabled={!savePresetName.trim()}>
				Save
			</button>
		</div>
	</Modal>
{/if}

{#if ctxMenu}
	<PosterContextMenu
		x={ctxMenu.x}
		y={ctxMenu.y}
		watched={ctxMenu.watched}
		contentId={ctxMenu.contentId}
		contentType={ctxMenu.contentType}
		mediaName={ctxMenu.mediaName}
		onClose={() => { ctxMenu = undefined; }}
		onWatchedUpdate={handleCtxMenuWatchedUpdate}
	/>
{/if}

<style lang="scss">
	.tierlist-page {
		padding: 24px;
		padding-bottom: 280px;
		max-width: 1400px;
		margin: 0 auto;
	}

	.tierlist-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 24px;
		flex-wrap: wrap;
		gap: 12px;

		h2 {
			margin: 0;
			font-size: 26px;
			font-weight: 700;
			letter-spacing: -0.3px;
		}
	}

	.tierlist-selector {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.tls-tab {
		width: auto !important;
		padding: 5px 14px;
		border-radius: 8px;
		border: 1.5px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.06);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 160ms ease;
		white-space: nowrap;

		&:hover {
			background: rgba(128, 128, 128, 0.16);
		}

		&.active {
			background: rgba(255, 255, 255, 0.1);
			border-color: rgba(255, 255, 255, 0.35);
			font-weight: 600;
		}
	}

	.tls-actions {
		display: flex;
		align-items: center;
		gap: 2px;
		margin-left: 2px;
	}

	.tls-icon {
		width: auto !important;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 4px 6px;
		border-radius: 6px;
		border: 1px solid transparent;
		background: transparent;
		color: $text-color;
		fill: $text-color;
		cursor: pointer;
		opacity: 0.4;
		transition: all 160ms ease;
		font-size: 15px;
		font-weight: 700;
		line-height: 1;

		&:hover {
			opacity: 1;
			background: rgba(128, 128, 128, 0.12);
			border-color: rgba(128, 128, 128, 0.2);
		}
	}

	.tls-icon-delete:hover {
		background: rgba(255, 70, 70, 0.12);
		border-color: rgba(255, 70, 70, 0.3);
		color: #ff5050;
		fill: #ff5050;
	}

	.tierlist-actions {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.btn-edit {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 18px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.3);
		background: rgba(128, 128, 128, 0.08);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 180ms ease;

		&:hover {
			background: rgba(128, 128, 128, 0.15);
			border-color: rgba(128, 128, 128, 0.4);
		}

		&.active {
			background: $accent-color-hover;
			color: $bg-color;
			border-color: $accent-color-hover;
		}
	}

	.btn-save {
		padding: 8px 22px;
		border-radius: 10px;
		border: none;
		background: $success;
		color: white;
		cursor: pointer;
		font-size: 13px;
		font-weight: 600;
		transition: all 180ms ease;
		box-shadow: 0 2px 8px rgba(40, 167, 69, 0.25);

		&:hover {
			background: $success-hover;
			box-shadow: 0 4px 12px rgba(40, 167, 69, 0.35);
		}

		&:disabled {
			opacity: 0.5;
			cursor: not-allowed;
			box-shadow: none;
		}
	}

	.btn-add {
		padding: 8px 18px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.3);
		background: rgba(128, 128, 128, 0.08);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 180ms ease;

		&:hover {
			background: rgba(128, 128, 128, 0.15);
			border-color: rgba(128, 128, 128, 0.4);
		}
	}

	.tierlist-container {
		display: flex;
		flex-direction: column;
		border-radius: 14px;
		overflow: hidden;
		border: 1px solid rgba(128, 128, 128, 0.12);
		box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15), 0 1px 3px rgba(0, 0, 0, 0.08);
		background: rgba(128, 128, 128, 0.02);
	}

	.tier-row {
		display: flex;
		min-height: 110px;
		border-bottom: 1px solid rgba(128, 128, 128, 0.1);
		transition: background-color 200ms ease;

		&:last-child {
			border-bottom: none;
		}

		&.drag-over {
			background-color: rgba(128, 128, 128, 0.06);
		}
	}

	.tier-label {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-width: 100px;
		width: auto;
		padding: 10px 12px;
		font-weight: 800;
		font-size: 28px;
		text-align: center;
		user-select: none;
		position: relative;
		gap: 4px;
		letter-spacing: -0.5px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
	}

	.tier-name {
		line-height: 1;
		word-break: break-all;
		overflow: hidden;
		max-width: 100%;
	}

	.tier-move-actions {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.tier-label-actions {
		display: flex;
		gap: 2px;
	}

	.tier-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 3px;
		border-radius: 5px;
		border: none;
		background: rgba(0, 0, 0, 0.15);
		color: inherit;
		cursor: pointer;
		opacity: 0.6;
		transition: all 150ms ease;

		&:hover {
			opacity: 1;
			background: rgba(0, 0, 0, 0.3);
		}

		&:disabled {
			opacity: 0.25;
			cursor: default;
			&:hover {
				background: rgba(0, 0, 0, 0.15);
			}
		}

		&.danger:hover {
			background: $error;
			color: white;
		}
	}

	.tier-items {
		display: flex;
		flex-flow: row wrap;
		align-items: flex-start;
		align-content: flex-start;
		flex: 1;
		padding: 8px;
		gap: 6px;
		min-height: 100px;
	}

	.tier-item {
		width: 68px;
		border-radius: 6px;
		cursor: pointer;
		transition: transform 200ms cubic-bezier(0.2, 0, 0, 1), opacity 200ms ease;
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		align-items: center;

		&.dragging {
			opacity: 0.15;
			transform: scale(0.92);
		}

		&:hover {
			transform: translateY(-3px) scale(1.03);

			.tier-item-poster {
				box-shadow: 0 4px 14px rgba(0, 0, 0, 0.3);
			}
		}

		a {
			display: block;
			width: 100%;
			height: 100%;
		}

		img {
			width: 100%;
			height: 100%;
			object-fit: cover;
			display: block;
		}

		@media screen and (max-width: 600px) {
			width: 50px;
		}
	}

	.drop-placeholder {
		width: 68px;
		height: 114px;
		border-radius: 6px;
		border: 2px dashed $accent-color-hover;
		background: rgba(128, 128, 128, 0.06);
		flex-shrink: 0;
		pointer-events: none;

		@media screen and (max-width: 600px) {
			width: 50px;
			height: 87px;
		}
	}

	.tier-item-poster {
		width: 100%;
		height: 100px;
		border-radius: 6px;
		overflow: hidden;
		box-shadow: 0 1px 6px rgba(0, 0, 0, 0.2);
		transition: box-shadow 200ms ease;

		@media screen and (max-width: 600px) {
			height: 74px;
		}
	}

	.tier-item-title {
		display: block;
		width: 100%;
		text-align: center;
		font-size: 9px;
		line-height: 1.2;
		color: $text-color;
		margin-top: 3px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		opacity: 0.7;
	}

	.no-poster {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(128, 128, 128, 0.15);
		color: $text-color;
		font-size: 10px;
		text-align: center;
		padding: 4px;
		word-break: break-word;
	}

	.tier-empty-hint {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		color: $placeholder-color;
		font-size: 13px;
		font-style: italic;
		opacity: 0.6;
	}

	.untiered-section {
		position: fixed;
		bottom: 0;
		left: 240px;
		right: 0;
		z-index: 50;
		background: $bg-color;
		border-top: 1px solid rgba(128, 128, 128, 0.2);
		box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.2);
		transition: opacity 200ms ease, transform 200ms ease, left 200ms ease;

		&.collapsed {
			.untiered-items {
				display: none;
			}
		}

		&.dragging:not(.collapsed) {
			opacity: 0.3;
		}

		h3 {
			margin: 0;
			font-weight: 600;
			font-size: 14px;
			opacity: 0.8;
			white-space: nowrap;
		}
	}

	.untiered-header {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 20px;
		user-select: none;
	}

	.untiered-filters {
		display: flex;
		flex-direction: row;
		align-items: center;
		gap: 6px;
		flex: 1;
		flex-wrap: wrap;
	}

	.filter-group {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-wrap: wrap;
	}

	.filter-sep {
		opacity: 0.2;
		font-size: 14px;
		user-select: none;
	}

	.filter-chip {
		width: auto !important;
		padding: 3px 10px;
		border-radius: 12px;
		border: 1px solid rgba(128, 128, 128, 0.25);
		background: rgba(128, 128, 128, 0.06);
		color: $text-color;
		cursor: pointer;
		font-size: 11px;
		font-weight: 500;
		white-space: nowrap;
		transition: all 150ms ease;
		opacity: 0.7;

		&:hover {
			opacity: 1;
			background: rgba(128, 128, 128, 0.12);
		}

		&.active {
			background: $accent-color-hover;
			color: $bg-color;
			border-color: $accent-color-hover;
			opacity: 1;
		}
	}

	.overlay-toggle {
		background: none;
		border: none;
		color: $text-color;
		cursor: pointer;
		padding: 4px 8px;
		opacity: 0.6;
		transition: opacity 150ms ease;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 32px !important;
		height: 32px;
		border-radius: 6px;
		margin-left: auto;

		&:hover {
			opacity: 1;
			background: rgba(128, 128, 128, 0.1);
		}
	}

	.overlay-hotzone {
		position: fixed;
		bottom: 0;
		left: 240px;
		right: 0;
		height: 48px;
		z-index: 49;
		transition: left 200ms ease;
	}

	:global(.sidebar-collapsed) .untiered-section {
		left: 64px;
	}
	:global(.sidebar-collapsed) .overlay-hotzone {
		left: 64px;
	}
	@media screen and (max-width: 768px) {
		.untiered-section {
			left: 0;
		}
		.overlay-hotzone {
			left: 0;
		}
	}

	.untiered-items {
		display: flex;
		flex-flow: row wrap;
		gap: 6px;
		padding: 12px 20px 16px;
		min-height: 80px;
		max-height: 220px;
		overflow-y: auto;
		border-top: 1px solid rgba(128, 128, 128, 0.1);
		transition: all 200ms ease;

		&.drag-over {
			background-color: rgba(128, 128, 128, 0.08);
			border-top-color: $accent-color-hover;
		}
	}

	.untiered-empty {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		padding: 16px;
		color: $placeholder-color;
		font-style: italic;
		font-size: 13px;
		opacity: 0.5;
	}

	/* Tier form in modals */
	.tier-form {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-width: 300px;

		label {
			display: flex;
			flex-direction: column;
			gap: 6px;
			font-weight: 500;
			font-size: 13px;
		}

		input[type="text"] {
			padding: 10px 14px;
			border-radius: 10px;
			border: 1.5px solid rgba(128, 128, 128, 0.25);
			background: $bg-color;
			color: $text-color;
			font-size: 14px;
			transition: border-color 150ms ease;

			&:focus {
				outline: none;
				border-color: $accent-color-hover;
			}
		}

		input[type="color"] {
			width: 42px;
			height: 42px;
			border: 2px solid rgba(128, 128, 128, 0.2);
			border-radius: 10px;
			cursor: pointer;
			padding: 2px;
		}

		button {
			padding: 11px 22px;
			border-radius: 10px;
			border: none;
			background: $accent-color-hover;
			color: $bg-color;
			cursor: pointer;
			font-size: 14px;
			font-weight: 600;
			transition: all 180ms ease;
			box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);

			&:hover {
				opacity: 0.92;
				box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
			}
		}
	}

	.color-picker-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.color-text {
		flex: 1;
	}

	.color-preview {
		padding: 8px 16px;
		border-radius: 8px;
		font-weight: bold;
		font-size: 18px;
		min-width: 60px;
		text-align: center;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
	}

	.preset-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 340px;
		max-width: 420px;
	}

	.preset-card {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 14px 16px;
		border-radius: 12px;
		border: 1.5px solid rgba(128, 128, 128, 0.15);
		background: rgba(128, 128, 128, 0.04);
		cursor: pointer;
		transition: all 180ms ease;
		text-align: left;
		color: $text-color;

		&:hover {
			border-color: $accent-color-hover;
			background: rgba(128, 128, 128, 0.08);
			transform: translateY(-1px);
			box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
		}
	}

	.preset-card-header {
		font-weight: 700;
		font-size: 15px;
		letter-spacing: -0.2px;
	}

	.preset-card-preview {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
	}

	.preset-tier-chip {
		padding: 3px 10px;
		border-radius: 6px;
		font-size: 12px;
		font-weight: 700;
		line-height: 1.4;
		white-space: nowrap;
	}

	.preset-card-desc {
		font-size: 11px;
		opacity: 0.5;
		line-height: 1.3;
	}

	.preset-note {
		margin-top: 12px;
		font-size: 11px;
		opacity: 0.45;
		line-height: 1.4;
		text-align: center;
	}

	/* Auto Color Gradient Modal */
	.gradient-modal {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-width: 340px;
	}

	.gradient-options {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-height: 280px;
		overflow-y: auto;
		padding-right: 4px;
		scrollbar-width: thin;
		scrollbar-color: rgba(155, 155, 155, 0.4) transparent;

		&::-webkit-scrollbar {
			width: 6px;
		}

		&::-webkit-scrollbar-track {
			background: transparent;
		}

		&::-webkit-scrollbar-thumb {
			background-color: rgba(155, 155, 155, 0.4);
			border-radius: 10px;

			&:hover {
				background-color: rgba(155, 155, 155, 0.6);
			}
		}
	}

	.gradient-option {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 12px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.04);
		color: $text-color;
		cursor: pointer;
		transition: all 150ms ease;
		text-align: left;

		&:hover {
			background: rgba(128, 128, 128, 0.1);
			border-color: rgba(128, 128, 128, 0.35);
		}

		&.selected {
			border-color: $accent-color-hover;
			background: rgba(128, 128, 128, 0.08);
		}

		span {
			font-size: 13px;
			font-weight: 500;
			white-space: nowrap;
		}
	}

	.gradient-swatch {
		display: flex;
		width: 80px;
		height: 24px;
		border-radius: 6px;
		overflow: hidden;
		flex-shrink: 0;
	}

	.gradient-preview-section {
		border-top: 1px solid rgba(128, 128, 128, 0.15);
		padding-top: 12px;
	}

	.gradient-preview-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.45;
		margin-bottom: 8px;
	}

	.gradient-preview-tiers {
		display: flex;
		flex-direction: column;
		border-radius: 10px;
		overflow: hidden;
		border: 1px solid rgba(128, 128, 128, 0.12);
	}

	.gradient-preview-tier {
		padding: 8px 16px;
		font-weight: 700;
		font-size: 16px;
		text-align: center;
		border-bottom: 1px solid rgba(0, 0, 0, 0.08);

		&:last-child {
			border-bottom: none;
		}
	}

	.gradient-actions {
		display: flex;
		gap: 8px;
		align-items: center;
		flex-wrap: wrap;
	}

	.btn-reverse {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.3);
		background: rgba(128, 128, 128, 0.08);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		width: auto;
		transition: all 180ms ease;

		&:hover {
			background: rgba(128, 128, 128, 0.15);
			border-color: rgba(128, 128, 128, 0.4);
		}

		&.active {
			border-color: $accent-color-hover;
			background: rgba(128, 128, 128, 0.12);
		}
	}

	.text-color-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.text-color-picker {
		width: 32px;
		height: 32px;
		padding: 0;
		border: 1.5px solid rgba(128, 128, 128, 0.3);
		border-radius: 8px;
		cursor: pointer;
		background: none;
	}

	.preset-section-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.45;
		margin-bottom: -4px;
	}

	.preset-card-wrapper {
		position: relative;
		display: flex;
		align-items: stretch;
		gap: 6px;

		.preset-card {
			flex: 1;
		}
	}

	.preset-delete-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 10px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.15);
		background: rgba(128, 128, 128, 0.04);
		color: $text-color;
		cursor: pointer;
		opacity: 0.5;
		transition: all 180ms ease;

		&:hover {
			opacity: 1;
			border-color: #f44336;
			color: #f44336;
			background: rgba(244, 67, 54, 0.08);
		}
	}

	.btn-save-preset {
		width: 100%;
		margin-top: 12px;
		padding: 10px 16px;
		border-radius: 10px;
		border: 1.5px dashed rgba(128, 128, 128, 0.3);
		background: rgba(128, 128, 128, 0.04);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 600;
		transition: all 180ms ease;

		&:hover {
			border-color: $accent-color-hover;
			background: rgba(128, 128, 128, 0.08);
		}
	}

	.save-preset-form {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-width: 320px;

		label {
			display: flex;
			flex-direction: column;
			gap: 4px;
			font-size: 12px;
			font-weight: 600;
			opacity: 0.7;
		}

		input {
			padding: 8px 12px;
			border-radius: 8px;
			border: 1.5px solid rgba(128, 128, 128, 0.25);
			background: rgba(128, 128, 128, 0.06);
			color: $text-color;
			font-size: 14px;
		}

		.save-preset-preview {
			display: flex;
			gap: 4px;
			flex-wrap: wrap;
		}
	}

	/* Touch drag: prevent scroll on items in edit mode */
	.tierlist-page.edit-mode .tier-item {
		touch-action: none;
	}

	/* Mobile: compact header buttons + tier layout */
	@media screen and (max-width: 600px) {
		.tierlist-page {
			padding: 12px 8px 280px;
		}

		.tierlist-header {
			flex-wrap: wrap;
			gap: 8px;

			h2 {
				font-size: 20px;
			}
		}

		.tierlist-selector {
			gap: 4px;
		}

		.tls-tab {
			padding: 4px 10px;
			font-size: 11px;
		}

		.tls-icon {
			padding: 3px 5px;
			font-size: 13px;
		}

		.tierlist-actions {
			flex-wrap: wrap;
			gap: 4px;
		}

		.btn-edit,
		.btn-add,
		.btn-save {
			padding: 6px 10px;
			font-size: 11px;
			border-radius: 8px;
		}

		.tier-label {
			min-width: 36px;
			width: 36px;
			padding: 3px 1px;
			/* Scale font: 1 char=16px, longer names shrink down to 9px */
			font-size: clamp(9px, calc(36px / var(--tier-name-len, 1) * 1.2), 16px);
		}

		.tier-move-actions {
			gap: 0;
		}

		.tier-btn {
			padding: 2px;
		}

		.tier-label-actions {
			gap: 1px;
		}

		.tier-items {
			padding: 4px;
			gap: 4px;
			min-height: 60px;
		}

		.tier-item {
			width: 44px;
		}

		.tier-item-poster {
			height: 65px;
		}

		.drop-placeholder {
			width: 44px;
			height: 78px;
		}

		.tier-item-title {
			font-size: 7px;
		}

		.untiered-header {
			padding: 8px 10px;
			gap: 6px;
			flex-wrap: wrap;
		}

		.untiered-filters {
			flex-wrap: wrap;
			gap: 3px 4px;
		}

		.filter-chip {
			padding: 2px 8px;
			font-size: 10px;
			flex: 0 0 auto;
		}

		.untiered-items {
			padding: 8px 10px 12px;
			gap: 4px;
			max-height: 180px;
		}

		.untiered-section h3 {
			font-size: 12px;
		}
	}
</style>
