package xyz.chambaz.flash.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.ShowChart
import androidx.compose.material3.*
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import xyz.chambaz.flash.model.DeckItem

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DeckListScreen(
    decks: List<DeckItem>,
    isRefreshing: Boolean,
    onRefresh: () -> Unit,
    onDeckSelected: (DeckItem) -> Unit,
    onStats: () -> Unit,
    onSettings: () -> Unit,
) {
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        "Flash",
                        style = MaterialTheme.typography.titleLarge.copy(
                            fontFeatureSettings = "smcp",
                            fontWeight = FontWeight.Bold,
                        ),
                        color = CurrentAccent,
                    )
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = CurrentAccent,
                    actionIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant,
                ),
                actions = {
                    IconButton(onClick = onStats) {
                        Icon(Icons.Default.ShowChart, contentDescription = "Stats")
                    }
                    IconButton(onClick = onSettings) {
                        Icon(Icons.Default.Settings, contentDescription = "Settings")
                    }
                },
            )
        },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = isRefreshing,
            onRefresh = onRefresh,
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            if (decks.isEmpty() && !isRefreshing) {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = androidx.compose.ui.Alignment.Center) {
                    Text("No decks on server", color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            } else {
                LazyColumn(modifier = Modifier.fillMaxSize()) {
                    items(decks) { deck ->
                        val hasDue = deck.due > 0
                        ListItem(
                            headlineContent = {
                                Text(
                                    deck.name,
                                    color = if (hasDue) MaterialTheme.colorScheme.onBackground
                                            else MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            },
                            supportingContent = {
                                Text(
                                    "${deck.due} due / ${deck.total} total",
                                    color = if (hasDue) CurrentAccent
                                            else MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            },
                            colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.background),
                            modifier = Modifier.clickable(enabled = hasDue) { onDeckSelected(deck) },
                        )
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    }
                }
            }
        }
    }
}
