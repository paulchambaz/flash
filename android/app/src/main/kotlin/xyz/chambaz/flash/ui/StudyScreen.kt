package xyz.chambaz.flash.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import xyz.chambaz.flash.model.CardState
import xyz.chambaz.flash.model.EvaluateResult
import java.time.Duration
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

enum class StudyState { Question, Evaluating, Result }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudyScreen(
    deckName: String,
    cards: List<CardState>,
    cardIndex: Int,
    deckTotal: Int,
    state: StudyState,
    result: EvaluateResult?,
    error: String?,
    submittedAnswer: String,
    threshold: Float,
    onSubmit: (answer: String) -> Unit,
    onContinue: () -> Unit,
    onBack: () -> Unit,
) {
    BackHandler { onBack() }

    val card = cards.getOrNull(cardIndex)
    val remaining = when (state) {
        StudyState.Result -> cards.size - cardIndex - 1
        else -> cards.size - cardIndex
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        "$remaining / $deckTotal",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                ),
            )
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 20.dp),
        ) {
            if (card == null) return@Column

            Spacer(Modifier.height(32.dp))
            Text(
                card.concept,
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(40.dp))

            when (state) {
                StudyState.Question -> key(cardIndex) { QuestionContent(onSubmit = onSubmit) }
                StudyState.Evaluating -> EvaluatingContent()
                StudyState.Result -> ResultContent(
                    card = card,
                    result = result,
                    error = error,
                    submittedAnswer = submittedAnswer,
                    threshold = threshold,
                    onContinue = onContinue,
                )
            }
        }
    }
}

@Composable
private fun QuestionContent(onSubmit: (String) -> Unit) {
    var answer by remember { mutableStateOf("") }

    Column(modifier = Modifier.fillMaxSize().imePadding()) {
        Text(
            "Your answer",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(
            value = answer,
            onValueChange = { answer = it },
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
        )
        Spacer(Modifier.height(16.dp))
        Button(
            onClick = { onSubmit(answer) },
            modifier = Modifier.fillMaxWidth(),
            enabled = answer.isNotBlank(),
        ) {
            Text("Submit")
        }
        Spacer(Modifier.height(16.dp))
    }
}

@Composable
private fun EvaluatingContent() {
    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(16.dp)) {
            CircularProgressIndicator(color = CurrentAccent)
            Text("Evaluating…", color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun ResultContent(
    card: CardState,
    result: EvaluateResult?,
    error: String?,
    submittedAnswer: String,
    threshold: Float,
    onContinue: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState()),
    ) {
        Text(
            "Reference",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(8.dp))
        ReferenceView(
            reference = card.reference,
            userAnswer = submittedAnswer,
            threshold = threshold.toDouble(),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(16.dp))
        Text(
            "Your answer",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(4.dp))
        Text(
            submittedAnswer,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onBackground,
        )

        HorizontalDivider(modifier = Modifier.padding(vertical = 16.dp), color = MaterialTheme.colorScheme.outlineVariant)

        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (error != null) {
                Text(error, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            } else if (result != null) {
                val color = if (result.correct) CurrentAccent else MaterialTheme.colorScheme.error
                Text(
                    if (result.correct) "Correct" else "Incorrect",
                    color = color,
                    style = MaterialTheme.typography.titleMedium,
                )
                Spacer(Modifier.height(4.dp))
                Text(
                    if (result.reshowInSession) "again this session"
                    else formatRelativeDue(result.nextDue),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }

        Spacer(Modifier.height(24.dp))
        Button(
            onClick = onContinue,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text("Continue")
        }
        Spacer(Modifier.height(16.dp))
    }
}

private fun formatRelativeDue(iso: String): String {
    return try {
        val dt = ZonedDateTime.parse(iso, DateTimeFormatter.ISO_DATE_TIME)
        val secs = Duration.between(ZonedDateTime.now(), dt).seconds.coerceAtLeast(0)
        when {
            secs < 60        -> "in ${secs}s"
            secs < 3_600     -> "in ${secs / 60}m"
            secs < 86_400    -> "in ${secs / 3_600}h"
            secs < 604_800   -> "in ${secs / 86_400}d"
            else             -> "in ${secs / 604_800}w"
        }
    } catch (_: Exception) {
        iso
    }
}

