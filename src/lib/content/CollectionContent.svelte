<script lang="ts">
	import type { Media, MediaCollection } from "@/types";
	import HorizontalList from "../HorizontalList.svelte";
	import Poster from "../poster/Poster.svelte";

	interface Props {
		collection: MediaCollection;
		parts: Media[];
	}

	let { collection, parts }: Props = $props();
</script>

{#if parts?.length > 0}
	<HorizontalList title={collection.name}>
		{#each parts as part, i}
			<Poster media={part} small={true} bind:watched={parts[i].watched} />
		{/each}
	</HorizontalList>
{/if}
