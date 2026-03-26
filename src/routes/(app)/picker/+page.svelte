<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import { goto } from "$app/navigation";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type PickerMode = "crate" | "wheel";
	type FilterMode = "status" | "tier";

	let items: Media[] = $state([]);
	let loading = $state(true);
	let error = $state("");
	let mode: PickerMode = $state("crate");
	let spinning = $state(false);
	let winner: Media | undefined = $state(undefined);
	let showResult = $state(false);

	// Filter mode
	let filterMode: FilterMode = $state("status");

	// Status filters
	let enabledStatuses: WatchedStatus[] = $state(["PLANNED", "WATCHING", "HOLD"]);

	// Tierlist filters
	let tiers: Tier[] = $state([]);
	let enabledTierIds: number[] = $state([]);
	let tiersLoaded = $state(false);

	async function loadTiers() {
		if (tiersLoaded) return;
		try {
			const r = await axios.get("/tierlist");
			tiers = r.data || [];
			tiersLoaded = true;
			if (tiers.length > 0) {
				enabledTierIds = tiers.map((t) => t.id);
			}
		} catch {
			tiers = [];
			tiersLoaded = true;
		}
	}

	function toggleTier(id: number) {
		if (enabledTierIds.includes(id)) {
			if (enabledTierIds.length <= 1) return;
			enabledTierIds = enabledTierIds.filter((x) => x !== id);
		} else {
			enabledTierIds = [...enabledTierIds, id];
		}
	}

	function switchFilterMode(m: FilterMode) {
		if (filterMode === m) return;
		filterMode = m;
		if (m === "tier") loadTiers();
	}

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) {
			if (enabledStatuses.length <= 1) return; // must have at least one
			enabledStatuses = enabledStatuses.filter((x) => x !== s);
		} else {
			enabledStatuses = [...enabledStatuses, s];
		}
	}

	// --- Audio (overlapping tick on every tile boundary) ---
	let audioCtx: AudioContext | undefined;
	let rollBuffer: AudioBuffer | undefined;

	async function initAudio() {
		if (audioCtx) return;
		audioCtx = new AudioContext();
		try {
			const resp = await fetch("/Roll.mp3");
			const buf = await resp.arrayBuffer();
			rollBuffer = await audioCtx.decodeAudioData(buf);
		} catch {
			// audio won't work, but don't block the spinner
		}
	}

	function playTick() {
		if (!audioCtx || !rollBuffer) return;
		const src = audioCtx.createBufferSource();
		src.buffer = rollBuffer;
		const gain = audioCtx.createGain();
		gain.gain.value = 0.1;
		src.connect(gain);
		gain.connect(audioCtx.destination);
		src.start();
	}

	// --- RAF tracking state ---
	let crateStripEl: HTMLDivElement | undefined = $state();
	let crateRafId = 0;
	let lastCrateSlot = -1;

	let wheelSvgEl: SVGSVGElement | undefined = $state();
	let wheelRafId = 0;
	let lastWheelSlice = -1;

	function trackCrateSound(itemW: number) {
		if (!crateStripEl) return;
		const style = getComputedStyle(crateStripEl);
		const matrix = new DOMMatrix(style.transform);
		const currentX = matrix.m41; // translateX value
		const containerW = crateStripEl.parentElement?.clientWidth ?? 600;
		const centerX = containerW / 2;
		const slotIndex = Math.floor((-currentX + centerX) / itemW);
		if (slotIndex !== lastCrateSlot && lastCrateSlot !== -1) {
			playTick();
		}
		lastCrateSlot = slotIndex;
		crateRafId = requestAnimationFrame(() => trackCrateSound(itemW));
	}

	function trackWheelSound(sliceAngle: number) {
		if (!wheelSvgEl) return;
		const style = getComputedStyle(wheelSvgEl);
		const matrix = new DOMMatrix(style.transform);
		const angle = Math.atan2(matrix.m21, matrix.m11) * (180 / Math.PI);
		const normAngle = ((angle % 360) + 360) % 360;
		const sliceIndex = Math.floor(normAngle / sliceAngle);
		if (sliceIndex !== lastWheelSlice && lastWheelSlice !== -1) {
			playTick();
		}
		lastWheelSlice = sliceIndex;
		wheelRafId = requestAnimationFrame(() => trackWheelSound(sliceAngle));
	}

	function stopTracking() {
		if (crateRafId) { cancelAnimationFrame(crateRafId); crateRafId = 0; }
		if (wheelRafId) { cancelAnimationFrame(wheelRafId); wheelRafId = 0; }
		lastCrateSlot = -1;
		lastWheelSlice = -1;
	}

	// --- Status counts ---
	let statusCounts: Record<string, number> = $state({});

	async function fetchStatusCounts() {
		const statuses: WatchedStatus[] = ["PLANNED", "WATCHING", "HOLD"];
		const counts: Record<string, number> = {};
		await Promise.all(
			statuses.map(async (s) => {
				try {
					const r = await axios.get("/watched", { params: { status: s, limit: 1, page: 1 } });
					counts[s] = r.data?.totalResults ?? 0;
				} catch {
					counts[s] = 0;
				}
			}),
		);
		statusCounts = counts;
	}

	// --- Crate mode state ---
	let crateItems: Media[] = $state([]);
	let crateOffset = $state(0);
	let crateTransition = $state("none");
	let crateContainerEl: HTMLDivElement | undefined = $state();

	// --- Wheel mode state ---
	let wheelRotation = $state(0);
	let wheelTransition = $state("none");

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w200${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) {
			return `https://images.igdb.com/igdb/image/upload/t_cover_big/${m.extPosterPath}.jpg`;
		}
		if (m.type === MediaTypeE.malManga) {
			return m.extPosterPath;
		}
		return "";
	}

	function getLink(m: Media): string | undefined {
		switch (m.type) {
			case MediaTypeE.tmdbMovie: return `/movie/${m.ids.tmdb}`;
			case MediaTypeE.tmdbShow: return `/tv/${m.ids.tmdb}`;
			case MediaTypeE.igdbGame: return `/game/${m.ids.igdb}`;
			case MediaTypeE.malManga: return `/manga/${m.ids.mal}`;
		}
	}

	async function loadItems() {
		loading = true;
		error = "";
		winner = undefined;
		showResult = false;
		try {
			if (filterMode === "tier") {
				await loadTiers();
				const watchedIds = new Set<number>();
				for (const tier of tiers) {
					if (enabledTierIds.includes(tier.id)) {
						for (const ti of tier.tierItems ?? []) {
							watchedIds.add(ti.watchedId);
						}
					}
				}
				if (watchedIds.size === 0) {
					items = [];
					loading = false;
					return;
				}
				// Load all watched to map by id, then build Media list
				const r = await axios.get("/watched", { params: { limit: 500, page: 1 } });
				const results: Media[] = r.data?.results ?? r.data ?? [];
				items = results.filter((m) => m.name && m.watched && watchedIds.has(m.watched.id));
			} else {
				const r = await axios.get("/watched", {
					params: {
						status: enabledStatuses.join(","),
						limit: 200,
						page: 1,
					},
				});
				const results = r.data?.results ?? r.data ?? [];
				items = results.filter((m: Media) => m.name);
			}
		} catch {
			error = "Failed to load your list.";
			items = [];
		}
		loading = false;
	}

	$effect(() => {
		void enabledStatuses;
		void enabledTierIds;
		void filterMode;
		loadItems();
	});

	function spin() {
		if (spinning || items.length < 2) return;
		spinning = true;
		showResult = false;
		winner = undefined;

		initAudio();

		const winnerIndex = Math.floor(Math.random() * items.length);
		winner = items[winnerIndex];

		if (mode === "crate") {
			startCrateSpin(winnerIndex);
		} else {
			startWheelSpin(winnerIndex);
		}
	}

	// --- CRATE MODE (CS2 style) ---
	function startCrateSpin(winnerIndex: number) {
		const ITEM_W = 130; // px per item (120 + 10 gap)
		const VISIBLE = crateContainerEl ? Math.floor(crateContainerEl.clientWidth / ITEM_W) : 5;
		const PRE_ITEMS = 40 + Math.floor(Math.random() * 20);
		const POST_ITEMS = Math.ceil(VISIBLE / 2) + 2;
		const strip: Media[] = [];

		for (let i = 0; i < PRE_ITEMS; i++) {
			strip.push(items[Math.floor(Math.random() * items.length)]);
		}
		strip.push(items[winnerIndex]);
		for (let i = 0; i < POST_ITEMS; i++) {
			strip.push(items[Math.floor(Math.random() * items.length)]);
		}

		crateItems = strip;
		crateOffset = 0;
		crateTransition = "none";

		requestAnimationFrame(() => {
			requestAnimationFrame(() => {
				const targetIndex = PRE_ITEMS;
				const containerW = crateContainerEl?.clientWidth ?? 600;
				const targetPos = targetIndex * ITEM_W - containerW / 2 + ITEM_W / 2;
				const subOffset = (Math.random() - 0.5) * 60;
				crateOffset = -(targetPos + subOffset);
				crateTransition = "transform 10s cubic-bezier(0.15, 0.85, 0.2, 1)";

				lastCrateSlot = -1;
				crateRafId = requestAnimationFrame(() => trackCrateSound(ITEM_W));

				setTimeout(() => {
					stopTracking();
					spinning = false;
					showResult = true;
				}, 10200);
			});
		});
	}

	// --- WHEEL MODE ---
	function startWheelSpin(winnerIndex: number) {
		const sliceAngle = 360 / items.length;
		const targetAngle = -(winnerIndex * sliceAngle) - sliceAngle / 2;
		const fullRotations = 5 + Math.floor(Math.random() * 3);
		const finalRotation = fullRotations * 360 + targetAngle + (Math.random() - 0.5) * (sliceAngle * 0.6);

		wheelRotation = 0;
		wheelTransition = "none";

		requestAnimationFrame(() => {
			requestAnimationFrame(() => {
				wheelRotation = finalRotation;
				wheelTransition = "transform 10s cubic-bezier(0.15, 0.85, 0.2, 1)";

				lastWheelSlice = -1;
				wheelRafId = requestAnimationFrame(() => trackWheelSound(sliceAngle));

				setTimeout(() => {
					stopTracking();
					spinning = false;
					showResult = true;
				}, 10200);
			});
		});
	}

	// Wheel helpers
	function polarToCartesian(cx: number, cy: number, r: number, angleDeg: number) {
		const rad = (angleDeg * Math.PI) / 180;
		return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
	}

	function describeArc(cx: number, cy: number, r: number, startAngle: number, endAngle: number) {
		const start = polarToCartesian(cx, cy, r, endAngle);
		const end = polarToCartesian(cx, cy, r, startAngle);
		const largeArc = endAngle - startAngle > 180 ? 1 : 0;
		return `M ${cx} ${cy} L ${start.x} ${start.y} A ${r} ${r} 0 ${largeArc} 0 ${end.x} ${end.y} Z`;
	}

	// Colors for wheel slices
	const wheelColors = [
		"#e74c3c", "#3498db", "#2ecc71", "#f39c12", "#9b59b6",
		"#1abc9c", "#e67e22", "#2980b9", "#27ae60", "#c0392b",
		"#8e44ad", "#16a085", "#d35400", "#2c3e50", "#f1c40f",
		"#7f8c8d", "#e91e63", "#00bcd4", "#ff5722", "#607d8b",
	];

	onMount(() => {
		fetchStatusCounts();
		return () => {
			stopTracking();
			audioCtx?.close();
		};
	});
</script>

<svelte:head>
	<title>Random Picker</title>
</svelte:head>

<div class="picker-page">
	<PageTitle title="Random Picker" />

	<!-- Filter mode toggle -->
	<div class="filter-mode-toggle">
		<button
			class="plain filter-mode-btn"
			class:active={filterMode === "status"}
			onclick={() => switchFilterMode("status")}
		>
			Status
		</button>
		<button
			class="plain filter-mode-btn"
			class:active={filterMode === "tier"}
			onclick={() => switchFilterMode("tier")}
		>
			Tierlist
		</button>
	</div>

	<!-- Status filters -->
	{#if filterMode === "status"}
		<div class="picker-filters">
			{#each [["PLANNED", "Planned"], ["WATCHING", "Watching"], ["HOLD", "On Hold"]] as [s, label]}
				<button
					class="plain filter-btn"
					class:active={enabledStatuses.includes(s as WatchedStatus)}
					onclick={() => toggleStatus(s as WatchedStatus)}
				>
					{label}{statusCounts[s] != null ? ` (${statusCounts[s]})` : ""}
				</button>
			{/each}
		</div>
	{:else}
		<div class="picker-filters">
			{#if !tiersLoaded}
				<SpinnerTiny />
			{:else if tiers.length === 0}
				<span class="no-tiers">No tiers found. Create a tierlist first.</span>
			{:else}
				{#each tiers as tier}
					<button
						class="plain filter-btn tier-filter-btn"
						class:active={enabledTierIds.includes(tier.id)}
						onclick={() => toggleTier(tier.id)}
						style="--tier-bg: {tier.color}; --tier-text: {tier.textColor};"
					>
						{tier.name}{tier.tierItems ? ` (${tier.tierItems.length})` : ""}
					</button>
				{/each}
			{/if}
		</div>
	{/if}

	<!-- Mode toggle -->
	<div class="mode-toggle">
		<button
			class="plain mode-btn"
			class:active={mode === "crate"}
			onclick={() => { mode = "crate"; }}
		>
			Crate Opening
		</button>
		<button
			class="plain mode-btn"
			class:active={mode === "wheel"}
			onclick={() => { mode = "wheel"; }}
		>
			Wheel Spin
		</button>
	</div>

	{#if loading}
		<div class="picker-loading"><Spinner /></div>
	{:else if error}
		<div class="picker-error">{error}</div>
	{:else if items.length < 2}
		<div class="picker-empty">
			<Icon i="reel" wh={60} />
			<p>You need at least 2 items in the selected categories to spin.</p>
		</div>
	{:else}
		<!-- CRATE MODE -->
		{#if mode === "crate"}
			<div class="crate-wrapper">
				<div class="crate-indicator"></div>
				<div class="crate-container" bind:this={crateContainerEl}>
					<div
						class="crate-strip"
						bind:this={crateStripEl}
						style="transform: translateX({crateOffset}px); transition: {crateTransition};"
					>
						{#each crateItems.length > 0 ? crateItems : items as item, i}
							<div class="crate-item" class:winner={showResult && item === winner}>
								{#if getPoster(item)}
									<img src={getPoster(item)} alt={item.name} />
								{:else}
									<div class="crate-item-placeholder">
										<span>{item.name?.slice(0, 2)}</span>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			</div>
		{/if}

		<!-- WHEEL MODE -->
		{#if mode === "wheel"}
			<div class="wheel-wrapper">
				<div class="wheel-pointer">▼</div>
				<svg
					viewBox="0 0 400 400"
					class="wheel-svg"
					bind:this={wheelSvgEl}
					style="transform: rotate({wheelRotation}deg); transition: {wheelTransition};"
				>
					{#each items as item, i}
						{@const sliceAngle = 360 / items.length}
						{@const startAngle = i * sliceAngle}
						{@const endAngle = startAngle + sliceAngle}
						{@const midAngle = startAngle + sliceAngle / 2}
						{@const textR = items.length <= 12 ? 140 : 155}
						{@const textPos = polarToCartesian(200, 200, textR, midAngle)}
						<path
							d={describeArc(200, 200, 195, startAngle, endAngle)}
							fill={wheelColors[i % wheelColors.length]}
							stroke="rgba(0,0,0,0.3)"
							stroke-width="1"
						/>
						{#if items.length <= 20}
							<text
								x={textPos.x}
								y={textPos.y}
								text-anchor="middle"
								dominant-baseline="central"
								fill="white"
								font-size={items.length <= 8 ? "11" : items.length <= 14 ? "9" : "7"}
								font-weight="600"
								transform="rotate({midAngle}, {textPos.x}, {textPos.y})"
							>
								{(item.name?.length ?? 0) > 18 ? item.name?.slice(0, 16) + "…" : item.name}
							</text>
						{/if}
					{/each}
				</svg>
			</div>
		{/if}

		<!-- Spin button -->
		<button
			class="plain spin-btn"
			onclick={spin}
			disabled={spinning}
		>
			{#if spinning}
				<SpinnerTiny />
				<span>Spinning...</span>
			{:else}
				<Icon i="dice" wh={24} />
				<span>Spin!</span>
			{/if}
		</button>

		<!-- RESULT -->
		{#if showResult && winner}
			<div class="result-overlay" role="presentation" onclick={() => { showResult = false; }}>
				<div class="result-card" onclick={(e) => e.stopPropagation()}>
					<h2>You should watch:</h2>
					{#if getPoster(winner)}
						<img class="result-poster" src={getPoster(winner)} alt={winner.name} />
					{/if}
					<h3 class="result-title">{winner.name}</h3>
					<div class="result-actions">
						{#if getLink(winner)}
							<button class="plain result-btn" onclick={() => goto(getLink(winner)!)}>
								View Details
							</button>
						{/if}
						<button class="plain result-btn secondary" onclick={spin}>
							<Icon i="dice" wh={18} />
							Spin Again
						</button>
						<button class="plain result-btn tertiary" onclick={() => { showResult = false; }}>
							Close
						</button>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style lang="scss">
	.picker-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		padding: 20px;
		width: 100%;
		max-width: 900px;
		margin: 0 auto;
	}

	.picker-filters {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: center;
	}

	.filter-mode-toggle {
		display: flex;
		gap: 4px;
		background: $accent-color;
		border-radius: 10px;
		padding: 3px;
	}

	.filter-mode-btn {
		padding: 6px 14px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: $text-color-accent;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;

		&.active {
			background: $accent-color-hover;
			color: $bg-color;
		}

		&:hover:not(.active) {
			color: $text-color;
		}
	}

	.no-tiers {
		font-size: 13px;
		color: $text-color-accent;
	}

	.filter-label {
		font-size: 14px;
		color: $text-color-accent;
		font-weight: 500;
	}

	.filter-btn {
		padding: 8px 14px;
		border-radius: 8px;
		border: 2px solid $text-color;
		background: $bg-color;
		color: $text-color;
		fill: $text-color;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;

		&:hover,
		&.active {
			background: $accent-color-hover;
			color: $bg-color;
			fill: $bg-color;
		}

		&.active {
			border-color: $bg-color;
		}
	}

	.tier-filter-btn {
		border-color: var(--tier-bg);
		background: transparent;
		color: $text-color;

		&:hover,
		&.active {
			background: var(--tier-bg);
			color: var(--tier-text);
			border-color: var(--tier-bg);
		}
	}

	.mode-toggle {
		display: flex;
		gap: 4px;
		background: $accent-color;
		border-radius: 10px;
		padding: 3px;
	}

	.mode-btn {
		padding: 8px 18px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: $text-color-accent;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;

		&.active {
			background: $accent-color-hover;
			color: $bg-color;
		}

		&:hover:not(.active) {
			color: $text-color;
		}
	}

	.picker-loading, .picker-error, .picker-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 60px 20px;
		color: $text-color-accent;
		fill: $text-color-accent;
		text-align: center;

		p {
			max-width: 300px;
			font-size: 14px;
			line-height: 1.5;
		}
	}

	.picker-error { color: $error; }

	// --- CRATE MODE ---

	.crate-wrapper {
		position: relative;
		width: 100%;
		overflow: hidden;
		border-radius: 12px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		padding: 20px 0;
	}

	.crate-indicator {
		position: absolute;
		top: 0;
		left: 50%;
		transform: translateX(-50%);
		width: 3px;
		height: 100%;
		background: $text-color;
		z-index: 2;
		box-shadow: 0 0 10px $accent-color-hover;

		&::before {
			content: "▼";
			position: absolute;
			top: -2px;
			left: 50%;
			transform: translateX(-50%);
			color: $text-color;
			font-size: 16px;
		}
	}

	.crate-container {
		overflow: hidden;
		width: 100%;
	}

	.crate-strip {
		display: flex;
		gap: 10px;
		will-change: transform;
	}

	.crate-item {
		flex-shrink: 0;
		width: 120px;
		height: 170px;
		border-radius: 8px;
		overflow: hidden;
		border: 2px solid $bg-color-accent;
		background: $accent-color;
		transition: border-color 300ms ease;

		img {
			width: 100%;
			height: 100%;
			object-fit: cover;
		}

		&.winner {
			border-color: $accent-color-hover;
			box-shadow: 0 0 20px $accent-color;
		}
	}

	.crate-item-placeholder {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		background: $bg-color-accent;
		font-size: 28px;
		font-weight: 700;
		color: $text-color-accent;
	}

	// --- WHEEL MODE ---

	.wheel-wrapper {
		position: relative;
		width: min(100%, 400px);
		aspect-ratio: 1;
	}

	.wheel-pointer {
		position: absolute;
		top: -12px;
		left: 50%;
		transform: translateX(-50%);
		z-index: 2;
		font-size: 28px;
		color: $text-color;
		filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));
		line-height: 1;
	}

	.wheel-svg {
		width: 100%;
		height: 100%;
		will-change: transform;
		filter: drop-shadow(0 4px 20px rgba(0,0,0,0.4));
	}

	// --- SPIN BUTTON ---

	.spin-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 14px 36px;
		border-radius: 10px;
		border: 2px solid $text-color;
		background: $bg-color;
		color: $text-color;
		fill: $text-color;
		font-size: 18px;
		font-weight: 700;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
		min-width: 160px;

		&:hover:not(:disabled) {
			background: $text-color;
			color: $bg-color;
			fill: $bg-color;
		}

		&:disabled {
			opacity: 0.5;
			cursor: not-allowed;
		}
	}

	// --- RESULT OVERLAY ---

	.result-overlay {
		position: fixed;
		inset: 0;
		z-index: 9999;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0,0,0,0.7);
		backdrop-filter: blur(8px);
		animation: result-fadein 300ms ease forwards;
	}

	@keyframes result-fadein {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.result-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		background: $bg-color;
		border: 1px solid $bg-color-accent;
		border-radius: 20px;
		padding: 32px;
		max-width: 360px;
		width: 90%;
		animation: result-pop 400ms ease forwards;

		h2 {
			margin: 0;
			font-size: 18px;
			color: $text-color-accent;
			font-weight: 500;
		}

		h3 {
			margin: 0;
		}
	}

	@keyframes result-pop {
		0% { transform: scale(0.8); opacity: 0; }
		100% { transform: scale(1); opacity: 1; }
	}

	.result-confetti {
		font-size: 48px;
		animation: confetti-bounce 600ms ease;
	}

	@keyframes confetti-bounce {
		0% { transform: scale(0); }
		50% { transform: scale(1.3); }
		100% { transform: scale(1); }
	}

	.result-poster {
		width: 160px;
		border-radius: 10px;
		box-shadow: 0 4px 20px rgba(0,0,0,0.5);
	}

	.result-title {
		font-size: 20px;
		font-weight: 700;
		text-align: center;
		color: $text-color;
	}

	.result-actions {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
	}

	.result-btn {
		width: 100%;
		padding: 10px;
		border-radius: 10px;
		border: 2px solid $text-color;
		background: $bg-color;
		color: $text-color;
		fill: $text-color;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		transition: background-color 150ms ease, color 150ms ease;

		&:hover {
			background: $text-color;
			color: $bg-color;
			fill: $bg-color;
		}

		&.secondary {
			border-color: $bg-color-accent;
			color: $text-color;
			fill: $text-color;

			&:hover {
				background: $text-color;
				color: $bg-color;
				fill: $bg-color;
				border-color: $text-color;
			}
		}

		&.tertiary {
			background: transparent;
			border-color: transparent;
			color: $text-color-accent;

			&:hover {
				background: transparent;
				color: $text-color;
			}
		}
	}
</style>
