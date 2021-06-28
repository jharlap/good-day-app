CREATE TABLE `reflections` (
    `team_id` varchar(255) NOT NULL,
    `user_id` varchar(255) NOT NULL,
    `date` datetime NOT NULL,
    `work_day_quality` enum('0-terrible', '1-bad', '2-ok', '3-good', '4-awesome') NULL,
    `work_other_people_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `help_other_people_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `interrupted_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `progress_goals_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `quality_work_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `lot_of_work_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `work_day_feeling` enum('0-tense', '1-stress', '2-sad', '3-bored', '4-calm', '5-serene', '6-happy', '7-excited') NULL,
    `stressful_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `breaks_amount` enum('0-none', '1-little', '2-some', '3-much', '4-most') NULL,
    `meeting_number` enum('0-none', '1-one', '2-two', '3-few', '4-many') NULL,
    `most_productive_time` enum('0-morning', '1-midday', '2-earlyAft', '3-lateAft', '4-nonwork', '5-equally') NULL,
    `least_productive_time` enum('0-morning', '1-midday', '2-earlyAft', '3-lateAft', '4-nonwork', '5-equally') NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP(),
    PRIMARY KEY (`team_id`, `user_id`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

