<script lang="ts">
	import { afterNavigate, goto } from "$app/navigation";
	import { page } from "$app/state";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import tooltip from "@/lib/actions/tooltip";
	import DetailedMenu from "@/lib/nav/DetailedMenu.svelte";
	import FaceMenu from "@/lib/nav/FaceMenu.svelte";
	import FilterMenu from "@/lib/nav/FilterMenu.svelte";
	import FollowingMenu from "@/lib/nav/FollowingMenu.svelte";
	import SortMenu from "@/lib/nav/SortMenu.svelte";
	import TagMenu from "@/lib/tag/TagMenu.svelte";
	import TextImportModal from "@/lib/TextImportModal.svelte";
	import { isTouch } from "@/lib/util/helpers";
	import { store, defaultSort } from "@/store.svelte";
	import { RatingSystem } from "@/types";
	import axios from "axios";
	import { onMount } from "svelte";
	interface Props {
		children?: import("svelte").Snippet;
	}

	let { children }: Props = $props();

	let navEl: HTMLElement | undefined = $state();
	let mainSearchEl: HTMLInputElement | undefined = $state();
	let searchTimeout: number;
	let subMenuShown = $state(false);
	let filterMenuShown = $state(false);
	let sortMenuShown = $state(false);
	let followingMenuShown = $state(false);
	let detailedMenuShown = $state(false);
	let tagMenuShown = $state(false);
	let textImportShown = $state(false);
	let scroll = window.scrollY;

	function handleProfileClick() {
		if (!localStorage.getItem("token")) {
			goto("/login");
		} else {
			closeAllSubMenus("sub");
			subMenuShown = !subMenuShown;
		}
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
					// If we are already on the search page, we can attempt
					// to keep any existing type filter on the next query.
					searchParams.set("type", currentSearchType);
				}
				// Enable autofocus before running `goto` because on chromium
				// the .focus() call won't work, even after a timeout.
				// Using autofocus seems to work. Disables after goto runs.
				// https://github.com/sbondCo/Watcharr/issues/169
				target.autofocus = true;
				goto(`/search?${searchParams.toString()}`).then(() => {
					// Use mainSearchEl if nav not split, otherwise use ev target.
					if (!document.body.classList.contains("split-nav") && mainSearchEl) {
						mainSearchEl.focus();
						mainSearchEl.autofocus = false;
					} else {
						target?.focus();
					}
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
		if (except !== "sub") subMenuShown = false;
		if (except !== "filter") filterMenuShown = false;
		if (except !== "sort") sortMenuShown = false;
		if (except !== "following") followingMenuShown = false;
		if (except !== "detailed") detailedMenuShown = false;
		if (except !== "tag") tagMenuShown = false;
	}

	/**
	 * Adds or removed `split-nav` tag to body depending
	 * on how big the main search bar is.
	 */
	function decideOnNavSplit() {
		// At ≤620px, CSS handles search visibility (inline hidden, .small shown).
		// No need for split-nav class.
		if (window.innerWidth <= 1050) {
			document.body.classList.remove("split-nav");
			return;
		}
		if (window.innerWidth <= 305) {
			document.body.classList.add("split-nav");
			return;
		}
		// Temporarily unsplit to measure the real search input width
		const wasSplit = document.body.classList.contains("split-nav");
		if (wasSplit) document.body.classList.remove("split-nav");
		const bigInput = navEl?.querySelector("input:not(.small)");
		if (bigInput) {
			const b = bigInput.getBoundingClientRect();
			console.debug("decideOnNavSplit: bigInput width:", b.width);
			if (b.width <= 45) {
				document.body.classList.add("split-nav");
				console.debug("decideOnNavSplit: Splitting nav.");
			} else {
				console.debug("decideOnNavSplit: Unsplitting nav.");
			}
		} else {
			console.warn("decideOnNavSplit: bigInput not found!", bigInput);
			if (wasSplit) document.body.classList.add("split-nav");
		}
	}

	function docOnScroll() {
		if (scroll > window.scrollY) {
			navEl?.classList.remove("scrolled-down");
			document.body.classList.add("nav-shown");
		} else {
			navEl?.classList.add("scrolled-down");
			document.body.classList.remove("nav-shown");
			closeAllSubMenus();
		}
		scroll = window.scrollY;
	}

	function focusSearch() {
		try {
			if (!mainSearchEl) {
				console.warn("focusSearch: mainSearchEl not defined!");
				return;
			}
			if (document.activeElement === mainSearchEl) {
				console.debug("focusSearch: mainSearchEl is already focused.");
				return;
			}
			mainSearchEl.focus();
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

	afterNavigate(() => {
		decideOnNavSplit();
		closeAllSubMenus();
	});

	onMount(() => {
		if (navEl) {
			decideOnNavSplit();
			window.addEventListener("resize", decideOnNavSplit);
			window.document.addEventListener("scroll", docOnScroll);
			window.document.addEventListener("keydown", handleGlobalKeybind);

			return () => {
				window.removeEventListener("resize", decideOnNavSplit);
				window.document.removeEventListener("scroll", docOnScroll);
				window.document.removeEventListener("keydown", handleGlobalKeybind);
			};
		} else {
			console.error(
				"navEl doesn't exist, failed to initialize up/down listener",
			);
		}
	});
</script>

<nav bind:this={navEl}>
	<div class="wrapper">
		<a href="/">
			<span class="large">Watcharr</span>
			<span class="small">W</span>
		</a>
		{#if isLoggedIn}
			<div class="search">
				<input
					bind:this={mainSearchEl}
					type="text"
					placeholder="Search"
					bind:value={store.searchQuery}
					onkeydown={handleSearch}
				/>
				<Icon i="search" wh={19} />
			</div>
		{:else}
			<div class="search"></div>
		{/if}
		<div class="btns">
			<!-- View toggle + Export -->
			{#if page.url?.pathname === "/" || page.url?.pathname.includes("/lists/")}
				<button
					class="plain other viewToggle"
					onclick={() => { store.viewMode = store.viewMode === "grid" ? "list" : "grid"; }}
					use:tooltip={{
						text: store.viewMode === "grid" ? "List View" : "Grid View",
						pos: "bot",
					}}
				>
					<Icon i={store.viewMode === "grid" ? "view-list" : "view-grid"} />
				</button>
			{/if}
			{#if page.url?.pathname === "/"}
				<button
					class="plain other exportBtn"
					onclick={() => {
						window.dispatchEvent(new CustomEvent("watcharr-export"));
					}}
					use:tooltip={{
						text: "Export",
						pos: "bot",
					}}
				>
					<Icon i="download" />
				</button>
			{/if}
			<!-- Detailed posters -->
			{#if page.url?.pathname === "/" || page.url?.pathname.startsWith("/search") || page.url?.pathname.includes("/lists/")}
				<button
					class="plain other detailedView"
					onclick={() => {
						closeAllSubMenus("detailed");
						detailedMenuShown = !detailedMenuShown;
					}}
					use:tooltip={{
						text: "Detailed View",
						pos: "bot",
						condition: !detailedMenuShown,
					}}
				>
					<Icon i="eye" />
					{#if store.activeFilters?.type?.length > 0 || store.activeFilters?.status?.length > 0}
						<div class="indicator"></div>
					{/if}
				</button>
				{#if detailedMenuShown}
					<DetailedMenu />
				{/if}
			{/if}
			<!-- Show on watched list and shared/followed watched lists -->
			{#if page.url?.pathname === "/" || page.url?.pathname.includes("/lists/") || page.url?.pathname.includes("/tag/")}
				<button
					class="plain other sort"
					onclick={() => {
						closeAllSubMenus("sort");
						sortMenuShown = !sortMenuShown;
					}}
					use:tooltip={{ text: "Sort", pos: "bot", condition: !sortMenuShown }}
				>
					<Icon i="sort" />
					<!-- Show indicator if not equal to default and second item in array is not falsy -->
					{#if store.activeSort?.length === 2 && store.activeSort[1] && JSON.stringify(store.activeSort) !== JSON.stringify(defaultSort)}
						<div class="indicator"></div>
					{/if}
				</button>
				<button
					class="plain other filter"
					onclick={() => {
						closeAllSubMenus("filter");
						filterMenuShown = !filterMenuShown;
					}}
					use:tooltip={{
						text: "Filter",
						pos: "bot",
						condition: !filterMenuShown,
					}}
				>
					<Icon i="filter" />
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
			{#if isLoggedIn}
				{#if store.userSettings?.ratingSystem === RatingSystem.Tierlist}
					{#if page.url?.pathname === "/tierlist"}
						<button
							class="plain other tierlist"
							onclick={() => goto("/")}
							use:tooltip={{ text: "Watched List", pos: "bot" }}
						>
							<Icon i="view-list" wh={24} />
						</button>
					{:else}
						<button
							class="plain other tierlist"
							onclick={() => goto("/tierlist")}
							use:tooltip={{ text: "Tierlist", pos: "bot" }}
						>
							<Icon i="star" wh={24} />
						</button>
					{/if}
				{/if}
				<button
					class="plain other tag"
					onclick={() => {
						closeAllSubMenus("tag");
						tagMenuShown = !tagMenuShown;
					}}
					use:tooltip={{ text: "Tags", pos: "bot", condition: !tagMenuShown }}
				>
					<Icon i="tag" />
				</button>
				{#if tagMenuShown}
					<TagMenu
						onTagClick={(tag) => {
							goto(`/tag/${tag.id}`);
							tagMenuShown = false;
						}}
						showManageBtn={true}
					/>
				{/if}
				<button
					class="plain other textImport"
					onclick={() => (textImportShown = !textImportShown)}
					use:tooltip={{ text: "Import List", pos: "bot" }}
				>
					<Icon i="document" wh={22} />
				</button>
				<button
					class="plain other discover"
					onclick={() => goto("/discover")}
					use:tooltip={{ text: "Discover", pos: "bot" }}
				>
					<Icon i="compass" wh={26} />
				</button>
				<button
					class="plain other following"
					onclick={() => {
						closeAllSubMenus("following");
						followingMenuShown = !followingMenuShown;
					}}
					use:tooltip={{
						text: "Following",
						pos: "bot",
						condition: !followingMenuShown,
					}}
				>
					<Icon i="people" wh={26} />
				</button>
				{#if followingMenuShown}
					<FollowingMenu close={() => (followingMenuShown = false)} />
				{/if}
				<button class="plain face" onclick={handleProfileClick}>:)</button>
				{#if subMenuShown}
					<FaceMenu />
				{/if}
			{/if}
		</div>
	</div>
	{#if isLoggedIn}
		<input
			class="small"
			type="text"
			placeholder="Search"
			bind:value={store.searchQuery}
			onkeydown={handleSearch}
		/>
	{/if}
</nav>

{#if textImportShown}
	<TextImportModal onClose={() => (textImportShown = false)} />
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

<style lang="scss">
	nav {
		display: flex;
		flex-flow: column;
		margin-bottom: 20px;
		padding: 10px 20px;
		position: sticky;
		top: 0;
		gap: 3px;
		z-index: 99990;
		transition: top 200ms ease-in-out;
		max-width: 100vw;
		@include nav-blur;

		&:global(.scrolled-down) {
			top: -110px;
		}

		.wrapper {
			display: flex;
			flex-flow: row;
			gap: 20px;
			justify-content: space-between;
			align-items: center;
			overflow-x: clip;

			a,
			.btns {
				/* This makes the logo on left and icons on right the same
				width, ensuring the main search bar can stay truly centered
				when possible. */
				flex: 1;
				min-width: 0;
			}

			@media screen and (max-width: 435px) {
				gap: 15px;
			}

			/* Slowly decrease the gap to ensure the main search bar doesn't get big enough again and pop back up in the nav. */
			body.split-nav & {
				@media screen and (max-width: 380px) {
					gap: 10px;
				}

				@media screen and (max-width: 375px) {
					gap: 8px;
				}

				@media screen and (max-width: 370px) {
					gap: 5px;
				}

				@media screen and (max-width: 350px) {
					gap: 0;
				}
			}
		}

		a {
			text-decoration: none;
			font-family:
				"Shrikhand",
				system-ui,
				-apple-system,
				BlinkMacSystemFont;
			font-size: 35px;
			transition:
				-webkit-text-stroke 150ms ease,
				color 150ms ease,
				font-weight 150ms ease;

			&:hover,
			&:focus-visible {
				color: $bg-color;
				-webkit-text-stroke: 3px $text-color;
				font-weight: bold;
			}

			span.large {
				display: block;
				width: 185.2px;
			}

			span.small {
				display: none;
				width: 40px;
			}

			@media screen and (max-width: 620px) {
				span.large {
					display: none;
				}
				span.small {
					display: block;
				}
			}
		}

		.search {
			width: 100%;
			position: relative;

			// Make the box look a little more centered, inline with the rest of the nav items.
			margin-bottom: 2px;

			:global(svg) {
				display: none;
				position: absolute;
				top: 50%;
				left: 50%;
				transform: translate(-50%, -50%);
				pointer-events: none;
				user-select: none;
			}

			input:focus-within + :global(svg),
			input:not(:placeholder-shown) + :global(svg) {
				display: none;
			}

			@media screen and (min-width: 666px) {
				max-width: 250px;
			}

			@media screen and (max-width: 666px) {
				& input:not(.small) {
					width: 100%;
				}

				&:focus-within + .btns button:not(.face) {
					display: none;
				}
			}

			// Hide inline search early — .small input below wrapper takes over
			@media screen and (max-width: 1050px) {
				display: none;
			}
		}

		:global(body.split-nav) & {
			.search {
				/* Collapse to zero width so it doesn't cause overflow.
				   decideOnNavSplit temporarily unsplits to measure. */
				flex: 0 0 0px;
				min-width: 0;
				overflow: hidden;
				opacity: 0;
				visibility: hidden;
			}

			input.small {
				display: block;
			}
		}

		input {
			width: 100%;
			font-weight: bold;
			text-align: center;
			box-shadow: 4px 4px 0px 0px $text-color;
			text-overflow: ellipsis;
			transition:
				width 150ms ease,
				box-shadow 150ms ease;

			&.small {
				display: none;
				margin-left: auto;
				margin-right: auto;

				// Show the below-wrapper search bar
				@media screen and (max-width: 1050px) {
					display: block;
					width: 100%;
				}
			}

			&:hover,
			&:focus {
				box-shadow: 2px 2px 0px 0px $text-color;
			}

			@media screen and (max-width: 290px) {
				&.small {
					width: 100%;
				}
			}
		}

		.btns {
			display: flex;
			flex-flow: row;
			justify-content: end;
			/* gap: 20px; */

			button.other {
				padding-top: 2px;
				width: 28px;
				flex-shrink: 0;

				@media screen and (max-width: 500px) {
					width: 24px;
				}

				@media screen and (max-width: 400px) {
					width: 22px;
				}

				transition:
					fill 150ms ease,
					stroke 150ms ease,
					stroke-width 150ms ease;
				fill: $text-color;

				&:hover,
				&:focus-visible {
					:global(path) {
						fill: none;
						stroke: $text-color;
						stroke-width: 30px;
						stroke-linejoin: round;
					}
				}
			}

			button.filter {
				&:hover,
				&:focus-visible {
					:global(path) {
						stroke-width: 15px;
					}
				}
			}

			button.viewToggle,
			button.exportBtn {
				&:hover,
				&:focus-visible {
					:global(path) {
						fill: $text-color;
						stroke: none;
					}
					opacity: 0.6;
				}
			}

			button.filter,
			button.sort {
				position: relative;

				.indicator {
					position: absolute;
					top: 1px;
					right: -6px;
					width: 6px;
					height: 6px;
					background-color: $text-color;
					border-radius: 50%;
				}
			}

			button.discover {
				transition:
					fill 150ms ease,
					stroke 150ms ease,
					stroke-width 150ms ease,
					transform 150ms ease;

				&:hover,
				&:focus-visible {
					transform: rotate(60deg);
				}
			}

			& > button:not(.face) {
				margin-right: 12px;

				@media screen and (max-width: 500px) {
					margin-right: 6px;
				}

				@media screen and (max-width: 400px) {
					margin-right: 3px;
				}
			}

			button.following {
				margin-right: 17px;

				@media screen and (max-width: 500px) {
					margin-right: 8px;
				}

				@media screen and (max-width: 400px) {
					margin-right: 4px;
				}
			}

			button.face {
				font-family:
					"Shrikhand",
					system-ui,
					-apple-system,
					BlinkMacSystemFont;
				font-size: 25px;
				transform: rotate(90deg);
				cursor: pointer;
				margin-left: 3px;
				transition:
					-webkit-text-stroke 150ms ease,
					color 150ms ease;

				&:hover,
				&:focus-visible {
					color: $bg-color;
					-webkit-text-stroke: 1.5px $text-color;
				}
			}
		}
	}
</style>
