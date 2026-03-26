<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Icon from "@/lib/Icon.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type FilterMode = "status" | "tier";

	let filterMode: FilterMode = $state("status");
	let enabledStatuses: WatchedStatus[] = $state(["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"]);
	let tiers: Tier[] = $state([]);
	let enabledTierIds: number[] = $state([]);
	let tiersLoaded = $state(false);
	let statusCounts: Record<string, number> = $state({});

	let allItems: Media[] = $state([]);
	let loading = $state(true);
	let error = $state("");
	let gamePhase: "setup" | "playing" | "gameover" = $state("setup");

	let roundItems: Media[] = $state([]);
	let userOrder: Media[] = $state([]);
	let correctOrder: Media[] = $state([]);
	let submitted = $state(false);
	let roundCorrect = $state(false);
	let score = $state(0);
	let round = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let usedKeys = new Set<string>();

	// Drag state
	let dragIdx = $state(-1);
	let dragOverIdx = $state(-1);
	let isDragging = $state(false);

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) { if (enabledStatuses.length <= 1) return; enabledStatuses = enabledStatuses.filter((x) => x !== s); }
		else { enabledStatuses = [...enabledStatuses, s]; }
	}
	async function loadTiers() {
		if (tiersLoaded) return;
		try { const r = await axios.get("/tierlist"); tiers = r.data || []; tiersLoaded = true; if (tiers.length > 0) enabledTierIds = tiers.map((t) => t.id); }
		catch { tiers = []; tiersLoaded = true; }
	}
	function toggleTier(id: number) {
		if (enabledTierIds.includes(id)) { if (enabledTierIds.length <= 1) return; enabledTierIds = enabledTierIds.filter((x) => x !== id); }
		else { enabledTierIds = [...enabledTierIds, id]; }
	}
	function switchFilterMode(m: FilterMode) { if (filterMode === m) return; filterMode = m; if (m === "tier") loadTiers(); }

	async function fetchStatusCounts() {
		const statuses: WatchedStatus[] = ["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"];
		const counts: Record<string, number> = {};
		await Promise.all(statuses.map(async (s) => {
			try { const r = await axios.get("/watched", { params: { status: s, limit: 1, page: 1 } }); counts[s] = r.data?.totalResults ?? 0; }
			catch { counts[s] = 0; }
		}));
		statusCounts = counts;
	}

	async function loadItems() {
		loading = true; error = "";
		try {
			if (filterMode === "tier") {
				await loadTiers();
				const ids = new Set<number>();
				for (const t of tiers) { if (enabledTierIds.includes(t.id)) for (const ti of t.tierItems ?? []) ids.add(ti.watchedId); }
				if (ids.size === 0) { allItems = []; loading = false; return; }
				const r = await axios.get("/watched", { params: { limit: 500, page: 1 } });
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name && m.watched && ids.has(m.watched.id));
			} else {
				const r = await axios.get("/watched", { params: { status: enabledStatuses.join(","), limit: 500, page: 1 } });
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name);
			}
		} catch { error = "Failed to load items."; allItems = []; }
		loading = false;
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr]; for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a;
	}

	function getContentType(m: Media): string {
		if (m.type === MediaTypeE.tmdbMovie) return "movie";
		if (m.type === MediaTypeE.tmdbShow) return "tv";
		if (m.type === MediaTypeE.igdbGame) return "game";
		return "movie";
	}

	function getKey(m: Media): string {
		return `${m.type}-${m.ids?.tmdb ?? m.ids?.igdb ?? m.name}`;
	}

	async function fetchDetails(m: Media): Promise<Media | null> {
		try {
			const ct = getContentType(m);
			const id = ct === "game" ? m.ids?.igdb : m.ids?.tmdb;
			if (!id) return null;
			const r = ct === "game" ? await axios.get(`/game/${id}`) : await axios.get(`/content/${ct}/${id}`);
			return r.data;
		} catch { return null; }
	}

	function parseDate(d: string | undefined): number {
		if (!d) return 0;
		return new Date(d).getTime() || 0;
	}

	async function setupRound(): Promise<boolean> {
		let pool = allItems.filter((m) => !usedKeys.has(getKey(m)));
		if (pool.length < 4) { usedKeys.clear(); pool = [...allItems]; }
		if (pool.length < 4) return false;

		const candidates = shuffle(pool);
		const picked: { item: Media; detail: Media; date: number }[] = [];

		for (const m of candidates) {
			if (picked.length >= 4) break;
			const detail = await fetchDetails(m);
			if (!detail) continue;
			const date = parseDate(detail.releaseDate);
			if (date === 0) continue;
			// Make sure dates are different enough to be interesting
			if (picked.some(p => p.date === date)) continue;
			picked.push({ item: m, detail, date });
		}

		if (picked.length < 4) return false;

		for (const p of picked) usedKeys.add(getKey(p.item));

		correctOrder = [...picked].sort((a, b) => a.date - b.date).map(p => p.item);
		roundItems = picked.map(p => p.item);
		userOrder = shuffle([...correctOrder]);
		submitted = false;
		roundCorrect = false;
		return true;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) { error = "You need at least 4 items."; return; }
		error = ""; score = 0; round = 0; streak = 0; bestStreak = 0;
		usedKeys.clear();
		if (!(await setupRound())) { error = "Couldn't find items with release dates."; return; }
		gamePhase = "playing";
	}

	function moveItem(from: number, to: number) {
		if (submitted || from === to) return;
		const arr = [...userOrder];
		const [item] = arr.splice(from, 1);
		arr.splice(to, 0, item);
		userOrder = arr;
	}

	function onDragStart(idx: number) {
		if (submitted) return;
		dragIdx = idx;
		isDragging = true;
	}

	function onDragEnter(idx: number) {
		if (dragIdx < 0 || idx === dragIdx) return;
		dragOverIdx = idx;
		// Immediately reorder while dragging for live preview
		moveItem(dragIdx, idx);
		dragIdx = idx;
	}

	function onDragEnd() {
		dragIdx = -1;
		dragOverIdx = -1;
		isDragging = false;
	}

	async function submitOrder() {
		if (submitted) return;
		submitted = true;

		const isCorrect = userOrder.every((m, i) => getKey(m) === getKey(correctOrder[i]));
		roundCorrect = isCorrect;

		if (isCorrect) {
			streak++;
			score += 200 + streak * 50;
			if (streak > bestStreak) bestStreak = streak;
			setTimeout(async () => {
				round++;
				if (!(await setupRound())) gamePhase = "gameover";
			}, 3000);
		} else {
			streak = 0;
			setTimeout(() => { gamePhase = "gameover"; }, 5000);
		}
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w92${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) return `https://images.igdb.com/igdb/image/upload/t_thumb/${m.extPosterPath}.jpg`;
		if (m.type === MediaTypeE.malManga) return m.extPosterPath;
		return "";
	}

	function getReleaseYear(m: Media): string {
		for (const c of correctOrder) {
			if (getKey(c) === getKey(m)) {
				// We stored the correct items — find its detail date from roundItems position
				return "";
			}
		}
		return "";
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			axios.post("/gamescore", { game: "timeline", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Release Timeline</title></svelte:head>

<div class="timeline-page">
	<PageTitle title="Release Timeline" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Sort these items from oldest to newest release date!</p>

		<div class="filter-mode-toggle">
			<button class="plain filter-mode-btn" class:active={filterMode === "status"} onclick={() => switchFilterMode("status")}>Status</button>
			<button class="plain filter-mode-btn" class:active={filterMode === "tier"} onclick={() => switchFilterMode("tier")}>Tierlist</button>
		</div>
		{#if filterMode === "status"}
			<div class="picker-filters">
				{#each [["PLANNED", "Planned"], ["WATCHING", "Watching"], ["HOLD", "On Hold"], ["FINISHED", "Finished"], ["DROPPED", "Dropped"]] as [s, label]}
					<button class="plain filter-btn" class:active={enabledStatuses.includes(s as WatchedStatus)} onclick={() => toggleStatus(s as WatchedStatus)}>
						{label}{statusCounts[s] != null ? ` (${statusCounts[s]})` : ""}
					</button>
				{/each}
			</div>
		{:else}
			<div class="picker-filters">
				{#if !tiersLoaded}<SpinnerTiny />
				{:else if tiers.length === 0}<span class="no-tiers">No tiers found.</span>
				{:else}
					{#each tiers as tier}
						<button class="plain filter-btn tier-filter-btn" class:active={enabledTierIds.includes(tier.id)} onclick={() => toggleTier(tier.id)} style="--tier-bg: {tier.color}; --tier-text: {tier.textColor};">
						{tier.name}{tier.tierItems ? ` (${tier.tierItems.length})` : ""}
					</button>
					{/each}
				{/if}
			</div>
		{/if}

		{#if error}<div class="error-msg">{error}</div>{/if}
		<button class="plain start-btn" onclick={startGame} disabled={loading}>
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="calendar" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		<p class="instruction">Drag to sort: Oldest → Newest</p>

		<div class="sort-list" class:dragging={isDragging}>
			{#each userOrder as item, i (getKey(item))}
				{@const poster = getPoster(item)}
				{@const isCorrectPos = submitted && getKey(item) === getKey(correctOrder[i])}
				{@const isWrongPos = submitted && getKey(item) !== getKey(correctOrder[i])}
				{@const isBeingDragged = dragIdx === i}
				<div
					class="sort-item"
					class:correct={isCorrectPos}
					class:wrong={isWrongPos}
					class:dragging-item={isBeingDragged}
					draggable={!submitted}
					ondragstart={() => onDragStart(i)}
					ondragenter={() => onDragEnter(i)}
					ondragover={(e) => e.preventDefault()}
					ondragend={onDragEnd}
				>
					<span class="sort-number">{i + 1}</span>
					{#if poster}<img src={poster} alt="" class="sort-poster" />{/if}
					<span class="sort-name">{item.name}</span>
					{#if submitted}
						<span class="sort-date">{correctOrder.find(c => getKey(c) === getKey(item))?.releaseDate?.split("T")[0] ?? ""}</span>
					{/if}
					{#if !submitted}
						<div class="sort-arrows">
							<button class="plain arrow-btn" disabled={i === 0} onclick={() => moveItem(i, i - 1)}>▲</button>
							<button class="plain arrow-btn" disabled={i === userOrder.length - 1} onclick={() => moveItem(i, i + 1)}>▼</button>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if !submitted}
			<button class="plain start-btn" onclick={submitOrder}>
				<Icon i="check" wh={18} /> Lock In
			</button>
		{:else}
			<div class="round-result" class:correct={roundCorrect} class:wrong={!roundCorrect}>
				{roundCorrect ? "✅ Perfect!" : "❌ Wrong order!"}
			</div>
		{/if}

	{:else if gamePhase === "gameover"}
		<div class="result-card">
			<div class="result-emoji">{bestStreak >= 10 ? "🏆" : bestStreak >= 5 ? "🌟" : bestStreak >= 3 ? "👍" : "💀"}</div>
			<h2>Game Over!</h2>
			<div class="result-stats">
				<div class="result-stat"><span class="result-stat-value">{score.toLocaleString()}</span><span class="result-stat-label">Total Score</span></div>
				<div class="result-stat"><span class="result-stat-value">{round} Rounds</span><span class="result-stat-label">Survived</span></div>
				<div class="result-stat"><span class="result-stat-value">🔥 {bestStreak}</span><span class="result-stat-label">Best Streak</span></div>
			</div>
			<div class="result-actions">
				<button class="plain start-btn" onclick={startGame}><Icon i="refresh" wh={18} /> Play Again</button>
				<a href="/games" class="plain back-link">Back to Games</a>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
	.timeline-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 600px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; }
	.filter-mode-toggle { display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px; }
	.filter-mode-btn { padding: 6px 14px; border-radius: 8px; border: none; background: transparent; color: $text-color-accent; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; } &:hover:not(.active) { color: $text-color; } }
	.picker-filters { display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; }
	.filter-btn { padding: 6px 12px; border-radius: 8px; border: 1px solid $bg-color-accent; background: transparent; color: $text-color-accent; font-size: 12px; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; border-color: $accent-color-hover; } }
	.tier-filter-btn {
		border-color: var(--tier-bg);
		&:hover, &.active { background: var(--tier-bg); color: var(--tier-text); border-color: var(--tier-bg); }
	}
	.error-msg { color: #ff6b6b; font-size: 14px; }
	.start-btn { display: flex; align-items: center; gap: 8px; padding: 12px 28px; border-radius: 12px; background: $accent-color-hover; color: $bg-color; fill: $bg-color; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 150ms ease, opacity 150ms ease; &:hover { transform: scale(1.03); } &:disabled { opacity: 0.5; cursor: not-allowed; } }
	.game-header { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 11px; color: $text-color-accent; font-weight: 600; text-transform: uppercase; }
	.stat-value { font-size: 20px; font-weight: 700; }
	.instruction { font-size: 14px; color: $text-color-accent; margin: 0; font-weight: 600; }
	.sort-list { display: flex; flex-direction: column; gap: 8px; width: 100%; &.dragging { user-select: none; } }
	.sort-item {
		display: flex; align-items: center; gap: 10px; padding: 10px 14px; border-radius: 12px;
		background: $accent-color; border: 2px solid $bg-color-accent; cursor: grab;
		transition: transform 300ms cubic-bezier(0.2, 0, 0, 1), box-shadow 200ms ease, border-color 200ms ease, background 200ms ease, opacity 200ms ease;
		&:active { cursor: grabbing; }
		&.dragging-item {
			opacity: 0.85;
			transform: scale(1.03);
			box-shadow: 0 8px 24px rgba(0,0,0,0.35);
			border-color: $accent-color-hover;
			z-index: 10;
			position: relative;
		}
		&.correct { border-color: #51cf66; background: rgba(81,207,102,0.15); animation: sort-pop 300ms ease; }
		&.wrong { border-color: #ff6b6b; background: rgba(255,107,107,0.15); animation: sort-shake 400ms ease; }
	}
	@keyframes sort-pop {
		0% { transform: scale(1); }
		50% { transform: scale(1.04); }
		100% { transform: scale(1); }
	}
	@keyframes sort-shake {
		0%, 100% { transform: translateX(0); }
		20% { transform: translateX(-6px); }
		40% { transform: translateX(6px); }
		60% { transform: translateX(-4px); }
		80% { transform: translateX(4px); }
	}
	.sort-number { font-size: 16px; font-weight: 700; color: $text-color-accent; min-width: 24px; text-align: center; }
	.sort-poster { width: 32px; height: 44px; object-fit: cover; border-radius: 4px; flex-shrink: 0; }
	.sort-name { font-size: 14px; font-weight: 600; color: $text-color; flex: 1; }
	.sort-date { font-size: 12px; color: $text-color-accent; font-weight: 600; }
	.sort-arrows { display: flex; flex-direction: column; gap: 2px; }
	.arrow-btn { padding: 2px 6px; font-size: 10px; background: $bg-color-accent; border-radius: 4px; cursor: pointer; color: $text-color; border: none; &:disabled { opacity: 0.3; cursor: default; } &:hover:not(:disabled) { background: $accent-color-hover; color: $bg-color; } }
	.round-result { font-size: 22px; font-weight: 700; &.correct { color: #51cf66; } &.wrong { color: #ff6b6b; } }
	.result-card { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 32px; border-radius: 16px; background: $accent-color; border: 1px solid $bg-color-accent; }
	.result-emoji { font-size: 64px; animation: result-bounce 500ms ease; }
	@keyframes result-bounce { 0% { transform: scale(0); } 60% { transform: scale(1.3); } 100% { transform: scale(1); } }
	.result-stats { display: flex; gap: 24px; flex-wrap: wrap; justify-content: center; }
	.result-stat { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.result-stat-value { font-size: 24px; font-weight: 700; color: $text-color; }
	.result-stat-label { font-size: 12px; color: $text-color-accent; font-weight: 600; }
	.result-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
	.back-link { color: $text-color-accent; font-size: 14px; text-decoration: none; &:hover { color: $text-color; } }
</style>
