<script lang="ts">
	import { onMount } from "svelte";

	interface Props {
		type?: "wrapped" | "vertical";
		children?: import("svelte").Snippet;
	}

	let { type = "wrapped", children }: Props = $props();

	let ulEl: HTMLUListElement = $state();

	onMount(() => {
		if (ulEl) {
			ulEl.classList.add(type);
		}
	});
</script>

<div>
	<ul bind:this={ulEl}>
		{@render children?.()}
	</ul>
</div>

<style lang="scss">
	div {
		display: flex;
		justify-content: center;
	}

	ul {
		display: flex;
		flex-flow: row;
		justify-content: center;
		gap: 10px;
		list-style: none;
		flex-wrap: wrap;
		margin: 20px 10px;
		max-width: 1200px;

		&:global(.vertical) {
			flex-wrap: nowrap;
			justify-content: unset;
			overflow-x: auto;
			padding: 15px 8px;
			margin: 5px 0;
		}

		// Desktop: fixed-width posters (only above mobile breakpoint)
		@media screen and (min-width: 601px) {
			&:global(.wrapped .container) {
				min-width: 150px !important;
				width: 170px !important;
				min-height: 256.367px !important;
			}
		}

		// Mobile: switch to CSS Grid for even columns that fill the screen.
		@media screen and (max-width: 600px) {
			&:global(.wrapped) {
				display: grid;
				grid-template-columns: repeat(3, 1fr);
				gap: 8px;
				margin: 12px 0;
				padding: 0 8px;
				width: 100%;
				max-width: 100%;
			}

			&:global(.wrapped .container) {
				min-width: 0 !important;
				width: 100% !important;
				min-height: 0 !important;
			}
		}

		@media screen and (max-width: 350px) {
			&:global(.wrapped) {
				grid-template-columns: repeat(2, 1fr);
			}
		}
	}
</style>
