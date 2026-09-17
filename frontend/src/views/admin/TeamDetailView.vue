<script setup lang="ts">
import TeamView from '@/views/user/TeamView.vue'
defineProps<{ adminTeamId: number }>()
</script>

<template><TeamView :key="adminTeamId" :admin-team-id="adminTeamId" /></template>
