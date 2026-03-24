<script lang="ts">
	import { afterNavigate, goto } from "$app/navigation";
	import { page } from "$app/state";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import tooltip from "@/lib/actions/tooltip";
	import DetailedMenu from "@/lib/nav/DetailedMenu.svelte";
	import FilterMenu from "@/lib/nav/FilterMenu.svelte";
	import FollowingMenu from "@/lib/nav/FollowingMenu.svelte";
	import SortMenu from "@/lib/nav/SortMenu.svelte";
	import TagMenu from "@/lib/tag/TagMenu.svelte";
	import TextImportModal from "@/lib/TextImportModal.svelte";
	import AboutModal from "@/lib/nav/AboutModal.svelte";
	import ProxyUserLogoutModal from "@/lib/logout/ProxyUserLogoutModal.svelte";
	import { isTouch } from "@/lib/util/helpers";
	import { parseTokenPayload, userHasPermission } from "@/lib/util/helpers";
	import { clearWatcharrData } from "@/lib/logout";
	import { notify } from "@/lib/util/notify";
	import { store, defaultSort } from "@/store.svelte";
	import { RatingSystem, UserPermission, UserType } from "@/types";
	import axios from "axios";
	import { onMount } from "svelte";
	interface Props {
		children?: import("svelte").Snippet;
	}

	let { children }: Props = $props();

	let searchEl: HTMLInputElement | undefined = $state();
	let searchTimeout: number;

	// Sidebar state
	let sidebarCollapsed = $state(localStorage.getItem("sidebarCollapsed") === "true");
	let mobileMenuOpen = $state(false);

	// Submenu states
	let filterMenuShown = $state(false);
	let sortMenuShown = $state(false);
	let detailedMenuShown = $state(false);
	let tagMenuShown = $state(false);
	let followingMenuShown = $state(false);
	let textImportShown = $state(false);
	let aboutModalOpen = $state(false);
	let proxyUserLogoutShown = $state(false);

	// Derived state
	let user = $derived(store.userInfo);
	let isAdmin = $derived(user ? userHasPermission(user.permissions, UserPermission.PERM_ADMIN) : false);
	let showContextToolbar = $derived(
		page.url?.pathname === "/" ||
		page.url?.pathname.includes("/lists/") ||
		page.url?.pathname.includes("/tag/") ||
		page.url?.pathname.startsWith("/search")
	);
	let showSortFilter = $derived(
		page.url?.pathname === "/" ||
		page.url?.pathname.includes("/lists/") ||
		page.url?.pathname.includes("/tag/")
	);
	let showViewExport = $derived(
		page.url?.pathname === "/" ||
		page.url?.pathname.includes("/lists/")
	);

	function toggleSidebar() {
		sidebarCollapsed = !sidebarCollapsed;
		localStorage.setItem("sidebarCollapsed", String(sidebarCollapsed));
	}

	function closeMobileMenu() {
		mobileMenuOpen = false;
	}

	function handleSearch(ev: KeyboardEvent) {
		if (
			ev.key === "ContextMenu" ||
			ev.key === "Home" ||
			ev.key === "End" ||
			ev.key === "PageDown" ||
			ev.key === "PageUp" ||
			ev.key === "NumLock" ||
			ev.key === "Escape" ||
			ev.key === "Tab" ||
			ev.key === "CapsLock" ||
			ev.key === "OS" ||
			ev.key === "ArrowLeft" ||
			ev.key === "ArrowRight" ||
			ev.key === "ArrowUp" ||
			ev.key === "ArrowDown" ||
			ev.key === "Control" ||
			ev.key === "Alt" ||
			ev.key === "AltGraph" ||
			ev.key === "Shift" ||
			ev.key === "Meta"
		)
			return;
		clearTimeout(searchTimeout);
		searchTimeout = window.setTimeout(
			() => {
				const target = ev.target as HTMLInputElement;
				const query = target?.value.trim();
				if (!query) return;
				const currentSearchType = page.url.searchParams.get("type");
				const searchParams = new URLSearchParams({
					query: encodeURIComponent(query),
					preferMyList: "true",
				});
				if (page.route?.id === "/(app)/search" && currentSearchType) {
					searchParams.set("type", currentSearchType);
				}
				target.autofocus = true;
				goto(`/search?${searchParams.toString()}`).then(() => {
					target?.focus();
					target.autofocus = false;
				});
			},
			isTouch() ? 800 : 400,
		);
	}

	let isLoggedIn = $state(!!localStorage.getItem("token"));

	async function getInitialData() {
		if (localStorage.getItem("token")) {
			const [u, s, f, fo, ts] = await Promise.all([
				axios.get("/user"),
				axios.get("/user/settings"),
				axios.get("/features"),
				axios.get("/follow"),
				axios.get("/tag"),
			]);
			if (u?.data) {
				store.userInfo = u.data;
			}
			if (s?.data) {
				store.userSettings = s.data;
			}
			if (f?.data) {
				store.serverFeatures = f.data;
			}
			if (fo?.data) {
				store.follows = fo.data;
			}
			if (ts?.data) {
				store.tags = ts.data;
			}
		} else if (!window.location.pathname.startsWith("/lists/")) {
			goto("/login?again=1");
		}
	}

	function closeAllSubMenus(except?: string) {
		if (except !== "filter") filterMenuShown = false;
		if (except !== "sort") sortMenuShown = false;
		if (except !== "detailed") detailedMenuShown = false;
		if (except !== "tag") tagMenuShown = false;
		if (except !== "following") followingMenuShown = false;
	}

	function focusSearch() {
		try {
			if (!searchEl) return;
			if (document.activeElement === searchEl) return;
			searchEl.focus();
		} catch (err) {
			console.error("focusSearch: Failed!", err);
		}
	}

	function handleGlobalKeybind(ev: KeyboardEvent) {
		switch (ev.key.toLowerCase()) {
			case "s":
				if (ev.ctrlKey) {
					ev.preventDefault();
					focusSearch();
				}
				break;
		}
	}

	function logout() {
		if (user?.type === UserType.Proxy) {
			proxyUserLogoutShown = true;
			return;
		}
		clearWatcharrData();
		goto("/login");
	}

	function shareWatchedList() {
		const nid = notify({ type: "loading", text: "Getting link" });
		const ud = parseTokenPayload();
		if (ud?.userId && ud?.username) {
			const shareLink = `${window.location.origin}/lists/${ud.userId}/${ud.username}`;
			navigator.clipboard
				.writeText(shareLink)
				.then(() => {
					notify({ id: nid, type: "success", text: "Copied share link" });
				})
				.catch((r) => {
					console.error("Failed to copy list share link", r);
					notify({
						id: nid,
						type: "error",
						text: `Failed to copy share link:<br/><a href="${shareLink}" target="_blank">${shareLink}</a>`,
						time: 20000,
					});
				});
		} else {
			notify({ id: nid, type: "error", text: "Failed to get link" });
		}
	}

	afterNavigate(() => {
		closeAllSubMenus();
		closeMobileMenu();
	});

	onMount(() => {
		// Clean up old nav classes from previous layout
		document.body.classList.remove("split-nav", "nav-shown");
		window.document.addEventListener("keydown", handleGlobalKeybind);
		return () => {
			window.document.removeEventListener("keydown", handleGlobalKeybind);
		};
	});
</script>

<!-- Mobile top bar -->
<header class="mobile-header">
	<button class="plain mobile-menu-toggle" onclick={() => (mobileMenuOpen = !mobileMenuOpen)}>
		<Icon i="menu" wh={22} />
	</button>
	<a href="/" class="mobile-logo">Watcharr</a>
	<button class="plain mobile-search-toggle" onclick={focusSearch}>
		<Icon i="search" wh={20} />
	</button>
</header>

<!-- Sidebar backdrop (mobile) -->
{#if mobileMenuOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="sidebar-backdrop" onclick={closeMobileMenu} onkeydown={() => {}}></div>
{/if}

<!-- Sidebar -->
<aside class="sidebar" class:collapsed={sidebarCollapsed} class:open={mobileMenuOpen}>
	<div class="sidebar-header">
		{#if !sidebarCollapsed}
			<a href="/" class="sidebar-logo">
				<span class="logo-full">Watcharr</span>
			</a>
		{/if}
		<button class="plain collapse-btn" onclick={toggleSidebar} use:tooltip={{ text: sidebarCollapsed ? "Expand" : "Collapse", pos: "right" }}>
			<Icon i="chevron" wh={18} facing={sidebarCollapsed ? "right" : undefined} />
		</button>
	</div>

	{#if isLoggedIn}
		<div class="sidebar-search">
			<div class="search-wrapper">
				<Icon i="search" wh={16} />
				<input
					bind:this={searchEl}
					type="text"
					placeholder="Search..."
					bind:value={store.searchQuery}
					onkeydown={handleSearch}
				/>
			</div>
		</div>

		<nav class="sidebar-nav">
			<a href="/" class="nav-item" class:active={page.url?.pathname === "/"} use:tooltip={{ text: "Home", pos: "right", condition: sidebarCollapsed }}>
				<Icon i="home" wh={20} />
				<span>Home</span>
			</a>
			<a href="/discover" class="nav-item" class:active={page.url?.pathname === "/discover"} use:tooltip={{ text: "Discover", pos: "right", condition: sidebarCollapsed }}>
				<Icon i="compass" wh={20} />
				<span>Discover</span>
			</a>
			{#if store.userSettings?.ratingSystem === RatingSystem.Tierlist}
				<a href="/tierlist" class="nav-item" class:active={page.url?.pathname === "/tierlist"} use:tooltip={{ text: "Tierlist", pos: "right", condition: sidebarCollapsed }}>
					<Icon i="star" wh={20} />
					<span>Tierlist</span>
				</a>
			{/if}
			<button class="plain nav-item" onclick={() => (textImportShown = !textImportShown)} use:tooltip={{ text: "Import", pos: "right", condition: sidebarCollapsed }}>
				<Icon i="document" wh={20} />
				<span>Import</span>
			</button>
			{#if page.url?.pathname === "/"}
				<button class="plain nav-item" onclick={() => { window.dispatchEvent(new CustomEvent("watcharr-export")); }} use:tooltip={{ text: "Export", pos: "right", condition: sidebarCollapsed }}>
					<Icon i="download" wh={20} />
					<span>Export</span>
				</button>
			{/if}

			<div class="nav-divider"></div>
			<div class="nav-label">Library</div>

			<button
				class="plain nav-item"
				class:active={tagMenuShown}
				onclick={() => { closeAllSubMenus("tag"); tagMenuShown = !tagMenuShown; }}
				use:tooltip={{ text: "Tags", pos: "right", condition: sidebarCollapsed }}
			>
				<Icon i="tag" wh={20} />
				<span>Tags</span>
				{#if store.tags?.length > 0}
					<span class="badge">{store.tags.length}</span>
				{/if}
			</button>
			{#if tagMenuShown}
				<div class="sidebar-submenu">
					<TagMenu
						onTagClick={(tag) => {
							goto(`/tag/${tag.id}`);
							tagMenuShown = false;
						}}
						showManageBtn={true}
						menuConfig={{ width: "200px", right: "unset", top: "0", arrowLeft: "unset", arrowRight: "unset" }}
					/>
				</div>
			{/if}

			<button
				class="plain nav-item"
				class:active={followingMenuShown}
				onclick={() => { closeAllSubMenus("following"); followingMenuShown = !followingMenuShown; }}
				use:tooltip={{ text: "Following", pos: "right", condition: sidebarCollapsed }}
			>
				<Icon i="people" wh={20} />
				<span>Following</span>
				{#if store.follows?.length > 0}
					<span class="badge">{store.follows.length}</span>
				{/if}
			</button>
			{#if followingMenuShown}
				<div class="sidebar-submenu">
					<FollowingMenu close={() => (followingMenuShown = false)} />
				</div>
			{/if}
		</nav>

		<div class="sidebar-footer">
			{#if !store.userSettings?.private}
				<button class="plain nav-item" onclick={shareWatchedList} use:tooltip={{ text: "Share List", pos: "right", condition: sidebarCollapsed }}>
					<Icon i="share" wh={18} />
					<span>Share List</span>
				</button>
			{/if}
			{#if isAdmin}
				<a href="/server" class="nav-item" class:active={page.url?.pathname === "/server"} use:tooltip={{ text: "Settings", pos: "right", condition: sidebarCollapsed }}>
					<Icon i="settings" wh={18} />
					<span>Settings</span>
				</a>
				<a href="/manage_users" class="nav-item" class:active={page.url?.pathname === "/manage_users"} use:tooltip={{ text: "Users", pos: "right", condition: sidebarCollapsed }}>
					<Icon i="people" wh={18} />
					<span>Users</span>
				</a>
				{#if store.serverFeatures?.sonarr || store.serverFeatures?.radarr}
					<a href="/arr_requests" class="nav-item" class:active={page.url?.pathname === "/arr_requests"} use:tooltip={{ text: "Requests", pos: "right", condition: sidebarCollapsed }}>
						<Icon i="ticket" wh={18} />
						<span>Requests</span>
					</a>
				{/if}
			{/if}

			<div class="nav-divider"></div>

			<a href="/profile" class="nav-item profile-item" class:active={page.url?.pathname === "/profile"} use:tooltip={{ text: user?.username ?? "Profile", pos: "right", condition: sidebarCollapsed }}>
				<span class="avatar">:)</span>
				<span class="profile-name">{user?.username ?? "Profile"}</span>
			</a>

			<button class="plain nav-item" onclick={logout} use:tooltip={{ text: "Logout", pos: "right", condition: sidebarCollapsed }}>
				<Icon i="logout" wh={18} />
				<span>Logout</span>
			</button>

			<div class="sidebar-about">
				<button class="about-link" onclick={() => (aboutModalOpen = !aboutModalOpen)}>about</button>
				<span class="about-sep">|</span>
				<a class="about-link" href="https://github.com/sbondCo/Watcharr/releases" target="_blank">
					v{__WATCHARR_VERSION__}
				</a>
			</div>
		</div>
	{/if}
</aside>

<!-- Main content -->
<div class="app-content" class:sidebar-collapsed={sidebarCollapsed}>
	{#if showContextToolbar && isLoggedIn}
		<div class="content-toolbar">
			<div class="toolbar-group">
				{#if showViewExport}
					<button
						class="toolbar-btn"
						onclick={() => { store.viewMode = store.viewMode === "grid" ? "list" : "grid"; }}
						use:tooltip={{ text: store.viewMode === "grid" ? "List View" : "Grid View", pos: "bot" }}
					>
						<Icon i={store.viewMode === "grid" ? "view-list" : "view-grid"} wh={18} />
					</button>
				{/if}
				{#if showSortFilter}
					<button
						class="toolbar-btn"
						class:active={sortMenuShown}
						onclick={() => { closeAllSubMenus("sort"); sortMenuShown = !sortMenuShown; }}
					>
						<Icon i="sort" wh={18} />
						<span>Sort</span>
						{#if store.activeSort?.length === 2 && store.activeSort[1] && JSON.stringify(store.activeSort) !== JSON.stringify(defaultSort)}
							<div class="indicator"></div>
						{/if}
					</button>
					<button
						class="toolbar-btn"
						class:active={filterMenuShown}
						onclick={() => { closeAllSubMenus("filter"); filterMenuShown = !filterMenuShown; }}
					>
						<Icon i="filter" wh={18} />
						<span>Filter</span>
						{#if store.activeFilters?.type?.length > 0 || store.activeFilters?.status?.length > 0}
							<div class="indicator"></div>
						{/if}
					</button>
					{#if sortMenuShown}
						<SortMenu />
					{/if}
					{#if filterMenuShown}
						<FilterMenu />
					{/if}
				{/if}
				{#if showContextToolbar}
					<button
						class="toolbar-btn"
						class:active={detailedMenuShown}
						onclick={() => { closeAllSubMenus("detailed"); detailedMenuShown = !detailedMenuShown; }}
					>
						<Icon i="eye" wh={18} />
						<span>Details</span>
					</button>
					{#if detailedMenuShown}
						<DetailedMenu />
					{/if}
				{/if}

			</div>
		</div>
	{/if}

	{#if textImportShown}
		<TextImportModal onClose={() => (textImportShown = false)} />
	{/if}

	{#if aboutModalOpen}
		<AboutModal onClose={() => (aboutModalOpen = false)} />
	{/if}

	{#if proxyUserLogoutShown}
		<ProxyUserLogoutModal onClose={() => (proxyUserLogoutShown = false)} />
	{/if}

	{#await getInitialData()}
		<Spinner />
	{:then}
		{@render children?.()}
	{:catch err}
		<Error
			pretty="Couldn't fetch app data!"
			error={err}
			onRetry={() => {
				location.reload();
			}}
		/>
	{/await}
</div>

<style lang="scss">
	/* ===== SIDEBAR ===== */
	.sidebar {
		position: fixed;
		top: 0;
		left: 0;
		width: 240px;
		height: 100dvh;
		display: flex;
		flex-direction: column;
		background-color: $bg-color;
		border-right: 1px solid rgba(128, 128, 128, 0.15);
		z-index: 99990;
		transition: width 200ms ease, transform 200ms ease;
		overflow: hidden;

		&.collapsed {
			width: 64px;

			.sidebar-header { justify-content: center; }
			.sidebar-search { padding: 0 8px; }
			.search-wrapper input { opacity: 0; width: 0; padding: 0; }
			.search-wrapper { justify-content: center; padding: 8px; }
			.nav-item span:not(.avatar) { display: none; }
			.nav-item .badge { display: none; }
			.nav-item { justify-content: center; padding: 10px; }
			.nav-label { display: none; }
			.sidebar-about { display: none; }
			.profile-item .profile-name { display: none; }
			.sidebar-submenu { display: none; }
		}

		@media screen and (max-width: 768px) {
			transform: translateX(-100%);
			width: 280px;
			box-shadow: none;

			&.open {
				transform: translateX(0);
				box-shadow: 4px 0 24px rgba(0, 0, 0, 0.5);
			}
		}
	}

	.sidebar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 14px 8px;
		min-height: 56px;
		flex-shrink: 0;

		.sidebar-logo {
			text-decoration: none;
			font-family: "Shrikhand", system-ui, -apple-system, BlinkMacSystemFont;
			font-size: 24px;
			color: $text-color;
			transition: opacity 150ms ease;
			white-space: nowrap;
			overflow: hidden;

			&:hover {
				opacity: 0.7;
			}
		}

		.collapse-btn {
			flex-shrink: 0;
			width: 28px;
			height: 28px;
			display: flex;
			align-items: center;
			justify-content: center;
			border-radius: 6px;
			border: none;
			background: none;
			opacity: 0.5;
			transition: opacity 150ms ease, background-color 150ms ease;

			&:hover, &:focus-visible {
				opacity: 1;
				background-color: rgba(128, 128, 128, 0.12);
				color: $text-color;
			}

			@media screen and (max-width: 768px) {
				display: none;
			}
		}
	}

	.sidebar-search {
		padding: 4px 12px 8px;
		flex-shrink: 0;

		.search-wrapper {
			display: flex;
			align-items: center;
			gap: 8px;
			background: rgba(128, 128, 128, 0.1);
			border: 1px solid rgba(128, 128, 128, 0.15);
			border-radius: 8px;
			padding: 8px 12px;
			transition: border-color 200ms ease, background-color 200ms ease;

			&:focus-within {
				border-color: rgba(128, 128, 128, 0.4);
				background: rgba(128, 128, 128, 0.15);
			}

			:global(svg) {
				flex-shrink: 0;
				opacity: 0.5;
			}

			input {
				width: 100%;
				background: none;
				border: none;
				outline: none;
				color: $text-color;
				font-size: 13px;
				font-weight: 500;
				padding: 0;
				box-shadow: none;
				text-align: left;

				&::placeholder {
					color: $placeholder-color;
				}

				&:hover, &:focus {
					box-shadow: none;
				}
			}
		}
	}

	.sidebar-nav {
		display: flex;
		flex-direction: column;
		padding: 4px 8px;
		gap: 2px;
		flex: 1;
		overflow-y: auto;
		scrollbar-width: thin;
		scrollbar-color: rgba(155, 155, 155, 0.3) transparent;

		&::-webkit-scrollbar { width: 4px; }
		&::-webkit-scrollbar-track { background: transparent; }
		&::-webkit-scrollbar-thumb {
			background-color: rgba(155, 155, 155, 0.3);
			border-radius: 10px;
		}
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 9px 14px;
		border-radius: 8px;
		color: $text-color;
		text-decoration: none;
		font-size: 13.5px;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 150ms ease, opacity 150ms ease;
		opacity: 0.7;
		white-space: nowrap;
		overflow: hidden;
		width: auto;
		border: none;
		background: none;
		text-align: left;
		justify-content: flex-start;

		:global(svg) {
			flex-shrink: 0;
			width: auto;
			height: auto;
		}

		&:hover, &:focus-visible {
			background-color: rgba(128, 128, 128, 0.1);
			color: $text-color;
			fill: $text-color;
			opacity: 1;
		}

		&.active {
			background-color: rgba(128, 128, 128, 0.15);
			color: $text-color;
			fill: $text-color;
			opacity: 1;
			font-weight: 600;
			border-color: transparent;
		}

		.badge {
			margin-left: auto;
			font-size: 11px;
			font-weight: 600;
			background: rgba(128, 128, 128, 0.15);
			padding: 1px 7px;
			border-radius: 10px;
			opacity: 0.7;
		}
	}

	.nav-divider {
		height: 1px;
		background: rgba(128, 128, 128, 0.12);
		margin: 6px 12px;
	}

	.nav-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.35;
		padding: 4px 14px 2px;
	}

	.sidebar-submenu {
		position: relative;
		margin-left: 12px;
		margin-bottom: 4px;

		:global(.menu) {
			position: relative;
			top: 0 !important;
			right: auto !important;
			left: 0;
			width: 100%;
			border: 1px solid rgba(128, 128, 128, 0.15);
			border-radius: 8px;

			:global(.arrow) {
				display: none;
			}
		}
	}

	.sidebar-footer {
		display: flex;
		flex-direction: column;
		padding: 4px 8px 12px;
		gap: 2px;
		flex-shrink: 0;
		border-top: 1px solid rgba(128, 128, 128, 0.1);

		.profile-item {
			.avatar {
				display: inline-flex;
				align-items: center;
				justify-content: center;
				width: 20px;
				height: 20px;
				font-family: "Shrikhand", system-ui, -apple-system, BlinkMacSystemFont;
				font-size: 11px;
				transform: rotate(90deg);
				background: rgba(128, 128, 128, 0.12);
				border-radius: 50%;
				flex-shrink: 0;
			}

			.profile-name {
				font-weight: 600;
				overflow: hidden;
				text-overflow: ellipsis;
			}
		}
	}

	.sidebar-about {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 14px;
		opacity: 0.35;
		font-size: 11px;

		.about-link {
			color: $text-color;
			text-decoration: none;
			cursor: pointer;
			background: none;
			border: none;
			padding: 0;
			font-size: 11px;
			font-weight: 500;
			width: auto;
			justify-content: flex-start;

			&:hover, &:focus-visible {
				opacity: 0.7;
				background: none;
				color: $text-color;
			}
		}

		.about-sep {
			opacity: 0.5;
		}
	}

	/* ===== MOBILE HEADER ===== */
	.mobile-header {
		display: none;
		position: sticky;
		top: 0;
		z-index: 99989;
		align-items: center;
		justify-content: space-between;
		padding: 10px 16px;
		@include nav-blur;

		.mobile-logo {
			font-family: "Shrikhand", system-ui, -apple-system, BlinkMacSystemFont;
			font-size: 22px;
			color: $text-color;
			text-decoration: none;
		}

		.mobile-menu-toggle,
		.mobile-search-toggle {
			width: auto;
			padding: 6px;
			border: none;
			background: none;
			opacity: 0.7;

			&:hover, &:focus-visible {
				opacity: 1;
				background: none;
				color: $text-color;
			}
		}

		@media screen and (max-width: 768px) {
			display: flex;
		}
	}

	.sidebar-backdrop {
		display: none;
		position: fixed;
		top: 0;
		left: 0;
		width: 100dvw;
		height: 100dvh;
		background: rgba(0, 0, 0, 0.5);
		z-index: 99989;
		backdrop-filter: blur(2px);

		@media screen and (max-width: 768px) {
			display: block;
		}
	}

	/* ===== MAIN CONTENT ===== */
	.app-content {
		margin-left: 240px;
		min-height: 100dvh;
		transition: margin-left 200ms ease;
		padding: 0 20px 20px;

		&.sidebar-collapsed {
			margin-left: 64px;
		}

		@media screen and (max-width: 768px) {
			margin-left: 0 !important;
		}
	}

	/* ===== CONTENT TOOLBAR ===== */
	.content-toolbar {
		display: flex;
		align-items: center;
		padding: 12px 0;
		position: sticky;
		top: 0;
		z-index: 100;
		@include nav-blur;

		@media screen and (max-width: 768px) {
			top: 48px;
		}

		.toolbar-group {
			display: flex;
			align-items: center;
			gap: 6px;
			flex-wrap: wrap;
			position: relative;

			:global(.menu) {
				top: 42px !important;
				right: auto !important;
				left: 0;
				z-index: 101;
				border: 1px solid rgba(128, 128, 128, 0.2) !important;
				border-radius: 10px;

				:global(.arrow) {
					display: none;
				}
			}
		}
	}

	.toolbar-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		border-radius: 8px;
		border: 1px solid rgba(128, 128, 128, 0.15);
		background: rgba(128, 128, 128, 0.06);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		width: auto;
		justify-content: center;
		transition: all 150ms ease;
		position: relative;

		:global(svg) {
			width: auto;
			height: auto;
		}

		&:hover, &:focus-visible {
			background: rgba(128, 128, 128, 0.12);
			border-color: rgba(128, 128, 128, 0.25);
			color: $text-color;
			fill: $text-color;
			opacity: 1;
		}

		&.active {
			background: rgba(128, 128, 128, 0.15);
			border-color: rgba(128, 128, 128, 0.3);
			color: $text-color;
		}

		span {
			@media screen and (max-width: 480px) {
				display: none;
			}
		}

		.indicator {
			position: absolute;
			top: 3px;
			right: 3px;
			width: 6px;
			height: 6px;
			background-color: $accent-color-hover;
			border-radius: 50%;
		}
	}
</style>
