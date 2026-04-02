<script lang="ts">
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import tooltip from "@/lib/actions/tooltip.js";
	import UserAvatar from "@/lib/img/UserAvatar.svelte";
	import { followUser, unfollowUser } from "@/lib/util/api.js";
	import { clearActiveFilters, store } from "@/store.svelte.js";
	import type { Media, MediaTypeE, PublicUser, Watched, Tier, Tierlist, SupportedMedia } from "@/types.js";
	import axios, { type GenericAbortSignal } from "axios";
	import { publicAxios, getToken } from "@/lib/util/api.js";
	import { onDestroy, untrack } from "svelte";
	import paginatedLoader, {
		PaginatedLoaderRunFnAction,
	} from "@/lib/util/paginatedLoader.svelte.js";
	import infScroll from "@/lib/util/infScroll.js";
	import { page } from "$app/state";
	import PosterList from "@/lib/poster/PosterList.svelte";
	import Poster from "@/lib/poster/Poster.svelte";
	import ListView from "@/lib/poster/ListView.svelte";
	import Error from "@/lib/Error.svelte";
	import { afterNavigate } from "$app/navigation";
	import { baseURL } from "@/lib/util/api.js";
	import PosterContextMenu from "@/lib/poster/PosterContextMenu.svelte";

	let meta = $derived.by(() => {
		return {
			id: page.params.id,
			username: page.params.username,
		};
	});

	let isLoggedIn = $derived(!!getToken());
	let followBtnDisabled = $state(false);
	let user: PublicUser | undefined = $state();
	let activeTab: "list" | "tierlist" = $state("list");
	let publicTierlists: Tierlist[] = $state([]);
	let activePublicTierlistId: number = $state(0);
	let activePublicTierlist = $derived(publicTierlists.find((t) => t.id === activePublicTierlistId));
	let publicTiers: Tier[] = $state([]);
	let tiersLoading = $state(false);
	let tiersError = $state(false);
	let tierlistContainer: HTMLDivElement | undefined = $state(undefined);

	// Map of own watched data keyed by "tmdbId-type" or "igdbId-game"
	let myWatchedMap: Map<string, Watched> = $state(new Map());

	function watchedKey(m: Media): string | undefined {
		if (m.type === "tmdb_movie" as MediaTypeE) return `${m.ids.tmdb}-movie`;
		if (m.type === "tmdb_tv" as MediaTypeE) return `${m.ids.tmdb}-tv`;
		if (m.type === "igdb_game" as MediaTypeE) return `${m.ids.igdb}-game`;
		return undefined;
	}

	function getMyWatched(m: Media): Watched | undefined {
		const key = watchedKey(m);
		return key ? myWatchedMap.get(key) : undefined;
	}

	async function loadMyWatchedData() {
		if (!isLoggedIn) return;
		try {
			const resp = await axios.get<Watched[]>("/watched");
			const map = new Map<string, Watched>();
			for (const w of resp.data) {
				if (w.content) {
					const type = w.content.type === "movie" ? "movie" : "tv";
					map.set(`${w.content.tmdbId}-${type}`, w);
				}
				if ((w as any).game) {
					map.set(`${(w as any).game.igdbId}-game`, w);
				}
			}
			myWatchedMap = map;
		} catch (err) {
			console.error("loadMyWatchedData: Failed!", err);
		}
	}

	async function loadPublicTierlists() {
		if (!meta.id || !meta.username) return;
		try {
			const resp = await publicAxios.get(`/tierlist/${meta.id}/${meta.username}/lists`);
			publicTierlists = resp.data || [];
			if (publicTierlists.length > 0 && !publicTierlists.find((t) => t.id === activePublicTierlistId)) {
				activePublicTierlistId = publicTierlists[0].id;
			}
		} catch {
			publicTierlists = [];
		}
	}

	async function loadPublicTierlist() {
		if (!meta.id || !meta.username) return;
		tiersLoading = true;
		tiersError = false;
		try {
			const resp = await publicAxios.get(`/tierlist/${meta.id}/${meta.username}`, {
				params: activePublicTierlistId ? { listId: activePublicTierlistId } : {},
			});
			publicTiers = resp.data || [];
		} catch {
			publicTiers = [];
			tiersError = true;
		}
		tiersLoading = false;
	}

	async function switchPublicTierlist(id: number) {
		activePublicTierlistId = id;
		await loadPublicTierlist();
	}

	function getTierItemPoster(item: any): string | undefined {
		const w = item.watched || item;
		if (w.content?.poster_path) return `${baseURL}/img${w.content.poster_path}`;
		if (w.game?.poster?.path) return `${baseURL}/${w.game.poster.path}`;
		if (w.game?.coverId) return `https://images.igdb.com/igdb/image/upload/t_cover_big/${w.game.coverId}.jpg`;
		if (w.manga?.poster?.path) return `${baseURL}/${w.manga.poster.path}`;
		if (w.manga?.posterUrl) return w.manga.posterUrl;
		return undefined;
	}

	function getTierItemTitle(item: any): string {
		const w = item.watched || item;
		if (w.content?.title) return w.content.title;
		if (w.game?.name) return w.game.name;
		if (w.manga?.title) return w.manga.title;
		return "Unknown";
	}

	function getTierItemLink(item: any): string | undefined {
		const w = item.watched || item;
		if (w.content) return `/${w.content.type}/${w.content.tmdbId}`;
		if (w.game) return `/game/${w.game.igdbId}`;
		if (w.manga) return `/manga/${w.manga.malId}`;
		return undefined;
	}

	// Context menu state for shared tierlist
	let ctxMenu: { x: number; y: number; watched: Watched | undefined; ownerWatched: Watched; contentId: number; contentType: SupportedMedia; mediaName: string } | undefined = $state(undefined);

	function getTierItemContentInfo(item: any): { contentId: number; contentType: SupportedMedia; mediaName: string } | undefined {
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

	function getTierItemOwnerWatched(item: any): Watched | undefined {
		if (item.watched) return item.watched;
		if (item.id && item.status !== undefined) return item as Watched;
		return undefined;
	}

	function getMyWatchedForTierItem(item: any): Watched | undefined {
		const w = item.watched || item;
		if (w.content) {
			const type = w.content.type === "movie" ? "movie" : "tv";
			return myWatchedMap.get(`${w.content.tmdbId}-${type}`);
		}
		if (w.game) {
			return myWatchedMap.get(`${w.game.igdbId}-game`);
		}
		return undefined;
	}

	function handleTierCtxMenu(e: MouseEvent, item: any) {
		if (!isLoggedIn) return;
		e.preventDefault();
		const info = getTierItemContentInfo(item);
		const ownerW = getTierItemOwnerWatched(item);
		if (!info || !ownerW) return;
		const myW = getMyWatchedForTierItem(item);
		ctxMenu = { x: e.clientX, y: e.clientY, watched: myW, ownerWatched: ownerW, ...info };
	}

	function handleCtxMenuUpdate(w: Watched | undefined) {
		loadMyWatchedData();
		ctxMenu = undefined;
	}

	// Equalize tier label widths
	$effect(() => {
		void publicTiers;
		if (!tierlistContainer) return;
		const labels = tierlistContainer.querySelectorAll<HTMLElement>(".tier-label");
		if (labels.length === 0) return;
		labels.forEach((l) => (l.style.minWidth = ""));
		requestAnimationFrame(() => {
			let maxW = 80;
			labels.forEach((l) => { if (l.offsetWidth > maxW) maxW = l.offsetWidth; });
			labels.forEach((l) => (l.style.minWidth = `${maxW}px`));
		});
	});

	let isFollowing = $derived(
		!!store.follows?.find((f) => f.followedUser.id === Number(meta.id)),
	);

	const scroll = infScroll({ callback: onScrollToBottom });
	const dataLoader = paginatedLoader<Media, undefined>(load);

	let nextLoadParams: {
		page: number;
		[x: string]: any;
	} = $derived({
		page: dataLoader.state.page + 1,
		...store.sortAndFiltersForQueryParams,
	});

	async function load(signal: GenericAbortSignal) {
		console.debug("load: loadParams:", nextLoadParams);
		if (nextLoadParams.page === dataLoader.state.page) {
			console.warn("load: Already on this page, not loading it again!");
			return;
		}
		if (!meta.id || !meta.username) {
			console.warn("load: Missing id or username!");
			return;
		}
		const r = await publicAxios.get(`/watched/${meta.id}/${meta.username}`, {
			params: nextLoadParams,
			signal,
		});
		scroll.dataLoaded();
		return r;
	}

	async function onScrollToBottom() {
		if (dataLoader.state.reqLoadError) {
			return;
		}
		console.debug("onScrollToBottom");
		dataLoader.runFn();
	}

	$effect(() => {
		if (store.sortAndFiltersForQueryParams) {
			untrack(() => {
				dataLoader.reset();
				dataLoader.runFn();
			});
		}
	});

	async function getPublicUser() {
		return (await publicAxios.get(`/user/public/${meta.id}/${meta.username}`))
			.data as PublicUser;
	}

	async function follow() {
		followBtnDisabled = true;
		console.log(isFollowing);
		if (isFollowing) {
			await unfollowUser(Number(meta.id));
		} else {
			await followUser(Number(meta.id));
		}
		followBtnDisabled = false;
	}

	$effect(() => {
		user = undefined;
		if (meta?.id && meta?.username) {
			getPublicUser()
				.then((u) => {
					user = u;
				})
				.catch((err) => {
					console.error("getPublicUser failed!", err);
				});
			loadMyWatchedData();
			loadPublicTierlists().then(() => loadPublicTierlist());
		}
	});

	afterNavigate((e) => {
		if (!e.from?.route?.id?.toLowerCase()?.includes("/lists")) {
			return;
		}
		console.log("afterNavigate.");
		dataLoader.abortReq("navigated away");
		dataLoader.runFn(PaginatedLoaderRunFnAction.Reset);
	});

	onDestroy(() => {
		console.debug("PAGE DESTROYED");
		scroll.destroy();
		dataLoader.abortReq("page destroyed");
	});

	// For ListView: overlay own watched data onto the shared list items
	let itemsWithMyWatched: Media[] = $derived(
		(dataLoader.state.data ?? []).map((m) => {
			if (!isLoggedIn) return m;
			const mine = getMyWatched(m);
			return mine ? { ...m, watched: mine } : { ...m, watched: undefined };
		}),
	);
</script>

<svelte:head>
	<title>{meta.username}'s Watched List</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<UserAvatar img={user?.avatar} />
		<div class="basic-ctr">
			<div class="name-row">
				<h2 title={user?.username}>
					{meta.username}
				</h2>
				{#if isLoggedIn}
					<button
						class="plain"
						disabled={followBtnDisabled}
						onclick={follow}
						use:tooltip={{ text: isFollowing ? "Unfollow" : "Follow" }}
					>
						<Icon i={isFollowing ? "person-minus" : "person-add"} />
					</button>
				{/if}
			</div>
			{#if user?.bio}
				<span title={user?.bio}>{user?.bio}</span>
			{/if}
		</div>
	</div>
</div>

{#if publicTierlists.length > 0 || publicTiers.length > 0 || !tiersError}
	<div class="view-tabs">
		<button
			class="view-tab"
			class:active={activeTab === "list"}
			onclick={() => (activeTab = "list")}
		>
			<Icon i="view-list" wh={16} />
			Watched List
		</button>
		<button
			class="view-tab"
			class:active={activeTab === "tierlist"}
			onclick={() => (activeTab = "tierlist")}
		>
			<Icon i="star" wh={16} />
			Tierlist
		</button>
	</div>
{/if}

{#if activeTab === "tierlist"}
	{#if publicTierlists.length > 1}
		<div class="public-tierlist-selector">
			{#each publicTierlists as tl (tl.id)}
				<button
					class="public-tierlist-tab"
					class:active={activePublicTierlistId === tl.id}
					onclick={() => switchPublicTierlist(tl.id)}
				>
					{tl.name}
				</button>
			{/each}
		</div>
	{/if}
	{#if tiersLoading}
		<Spinner />
	{:else if publicTiers.length > 0}
		<div class="public-tierlist" bind:this={tierlistContainer}>
			{#each publicTiers as tier (tier.id)}
				<div class="tier-row">
					<div
						class="tier-label"
						style="background-color: {tier.color}; color: {tier.textColor};"
					>
						<span class="tier-name">{tier.name}</span>
					</div>
					<div class="tier-items">
						{#if tier.tierItems && tier.tierItems.length > 0}
							{#each tier.tierItems as item}
								{@const poster = getTierItemPoster(item)}
								{@const title = getTierItemTitle(item)}
								{@const link = getTierItemLink(item)}
								<a href={link} class="tier-item" title={title} oncontextmenu={(e) => handleTierCtxMenu(e, item)}>
									<div class="tier-item-poster">
										{#if poster}
											<img src={poster} alt={title} loading="lazy" />
										{:else}
											<div class="no-poster">{title}</div>
										{/if}
									</div>
									<span class="tier-item-title">{title}</span>
								</a>
							{/each}
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="empty-list-wrap">
			<div class="empty-list">
				<Icon i="star" wh={80} />
				<h2 class="norm">No tierlist yet!</h2>
				<h4 class="norm">This user hasn't set up a tierlist.</h4>
			</div>
		</div>
	{/if}
{:else}

{#if store.viewMode === "list"}
	{#if dataLoader.state.data?.length > 0}
		<ListView items={itemsWithMyWatched} ownerItems={dataLoader.state.data} ownerName={meta.username} onWatchedUpdate={() => loadMyWatchedData()} />
	{:else if !dataLoader.state.reqLoading && !dataLoader.state.reqLoadError}
		<div class="empty-list-wrap">
			<div class="empty-list">
				<Icon i={store.hasActiveFilters ? "filter-circle" : "reel"} wh={80} />
				<h2 class="norm">This list is empty!</h2>
				<h4 class="norm">Come back later to see if they have added anything.</h4>
				{#if store.hasActiveFilters}
					<button onclick={() => clearActiveFilters()}>Clear Filters</button>
				{/if}
			</div>
		</div>
	{/if}
{:else}
	<PosterList>
		{#if dataLoader.state.data?.length > 0}
			{#each dataLoader.state.data as w, i (`${i}-${w.type}`)}
				{#if w}
					<Poster
						watched={getMyWatched(w)}
						media={w}
						fluidSize={true}
						disableInteraction={!isLoggedIn}
						ownerWatched={w.watched}
						ownerName={meta.username}
						onUpdated={() => loadMyWatchedData()}
					/>
				{/if}
			{/each}
		{:else if !dataLoader.state.reqLoading && !dataLoader.state.reqLoadError}
			<div class="empty-list">
				<Icon i={store.hasActiveFilters ? "filter-circle" : "reel"} wh={80} />
				<h2 class="norm">This list is empty!</h2>
				<h4 class="norm">Come back later to see if they have added anything.</h4>
				{#if store.hasActiveFilters}
					<button onclick={() => clearActiveFilters()}>Clear Filters</button>
				{/if}
			</div>
		{/if}
	</PosterList>
{/if}

{#if dataLoader.state.reqLoading}
	<div style="margin-bottom: 60px;">
		<Spinner />
	</div>
{/if}

{#if dataLoader.state.reqLoadError}
	<div style="margin-bottom: 60px;">
		<Error
			pretty="Failed to load results!"
			error={dataLoader.state.reqLoadError}
			onRetry={() => {
				dataLoader.state.reqLoadError = undefined;
				dataLoader.runFn();
			}}
		/>
	</div>
{/if}

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
		onWatchedUpdate={handleCtxMenuUpdate}
		ownerWatched={ctxMenu.ownerWatched}
		ownerName={meta.username}
	/>
{/if}

<style lang="scss">
	.content {
		display: flex;
		width: 100%;
		justify-content: center;

		.inner {
			display: flex;
			flex-flow: row;
			gap: 15px;
			justify-content: center;
			align-items: center;
			width: 100%;
			max-width: 1200px;
			margin: 20px 30px;
			margin-top: 0;
		}
	}

	button {
		width: max-content;
	}

	textarea {
		border: 0;
		padding: 0;
		resize: none;
		text-overflow: ellipsis;
	}

	.basic-ctr {
		min-width: 200px;
		max-width: 300px;
		overflow: hidden;

		.name-row {
			display: flex;
			flex-flow: row;
			gap: 15px;

			h2 {
				overflow: hidden;
				text-overflow: ellipsis;
			}

			button {
				margin-left: auto;
				fill: $text-color;
			}
		}

		span {
			font-family: monospace;
			overflow: hidden;
			text-overflow: ellipsis;
			display: -webkit-box;
			-webkit-line-clamp: 2;
			-webkit-box-orient: vertical;
		}
	}

	.empty-list {
		display: flex;
		flex-flow: column;
		gap: 5px;
		align-items: center;
		max-width: 400px;

		h2 {
			margin-top: 10px;
		}

		h4 {
			font-weight: normal;
			text-align: center;
		}

		button {
			width: max-content;
			padding-left: 20px;
			padding-right: 20px;
			margin-top: 15px;
		}
	}

	.empty-list-wrap {
		display: flex;
		justify-content: center;
	}

	.view-tabs {
		display: flex;
		justify-content: center;
		gap: 4px;
		margin: 0 20px 16px;
	}

	.view-tab {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 18px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.05);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 180ms ease;
		opacity: 0.6;

		&:hover {
			opacity: 0.85;
			background: rgba(128, 128, 128, 0.1);
		}

		&.active {
			opacity: 1;
			border-color: $accent-color;
			background: rgba($accent-color, 0.1);
		}
	}

	.public-tierlist-selector {
		display: flex;
		justify-content: center;
		gap: 4px;
		margin: 0 20px 16px;
		flex-wrap: wrap;
	}

	.public-tierlist-tab {
		padding: 5px 14px;
		border-radius: 8px;
		border: 1.5px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.05);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 180ms ease;
		opacity: 0.6;

		&:hover {
			opacity: 0.85;
			background: rgba(128, 128, 128, 0.1);
		}

		&.active {
			opacity: 1;
			border-color: rgba(255, 255, 255, 0.35);
			background: rgba(255, 255, 255, 0.12);
			font-weight: 600;
		}
	}

	.public-tierlist {
		max-width: 1400px;
		margin: 0 auto;
		padding: 0 24px 24px;
	}

	.tier-row {
		display: flex;
		border-bottom: 1px solid rgba(128, 128, 128, 0.12);

		&:first-child {
			border-radius: 12px 12px 0 0;
			overflow: hidden;
		}

		&:last-child {
			border-bottom: none;
			border-radius: 0 0 12px 12px;
			overflow: hidden;
		}
	}

	.tier-label {
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 80px;
		width: auto;
		padding: 10px 12px;
		font-weight: 800;
		font-size: 24px;
		text-align: center;
		user-select: none;
		letter-spacing: -0.5px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);

		@media screen and (max-width: 600px) {
			min-width: 50px;
			font-size: 18px;
			padding: 5px 6px;
		}
	}

	.tier-name {
		line-height: 1;
	}

	.tier-items {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		padding: 6px;
		flex: 1;
		align-items: flex-start;
		align-content: flex-start;
		min-height: 80px;
		background: rgba(128, 128, 128, 0.03);
	}

	.tier-item {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 62px;
		text-decoration: none;
		color: $text-color;
		transition: transform 120ms ease;

		&:hover {
			transform: translateY(-2px);
		}
	}

	.tier-item-poster {
		width: 62px;
		height: 93px;
		border-radius: 6px;
		overflow: hidden;
		background: rgba(128, 128, 128, 0.1);

		img {
			width: 100%;
			height: 100%;
			object-fit: cover;
		}

		@media screen and (max-width: 600px) {
			width: 50px;
			height: 75px;
		}
	}

	.no-poster {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 9px;
		text-align: center;
		padding: 4px;
		word-break: break-word;
		opacity: 0.5;
	}

	.tier-item-title {
		font-size: 9px;
		margin-top: 2px;
		text-align: center;
		width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		opacity: 0.7;
	}
</style>
