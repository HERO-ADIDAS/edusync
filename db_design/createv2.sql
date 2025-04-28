-- Drop and create the database
DROP DATABASE IF EXISTS edusync_db;
CREATE DATABASE edusync_db;
USE edusync_db;

-- Create USER table
CREATE TABLE user (
    user_id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    contact_number VARCHAR(20),
    profile_picture VARCHAR(255),
    role ENUM('teacher', 'student') NOT NULL,
    org VARCHAR(100),
    archive_delete_flag BOOLEAN DEFAULT TRUE
);

-- Create TEACHER table
CREATE TABLE teacher (
    teacher_id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    dept VARCHAR(100),
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES user(user_id) ON DELETE CASCADE
);

-- Create STUDENT table
CREATE TABLE student (
    student_id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    grade_level VARCHAR(20),
    enrollment_year INT,
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES user(user_id) ON DELETE CASCADE
);

-- Create CLASSROOM table
CREATE TABLE classroom (
    course_id INT PRIMARY KEY AUTO_INCREMENT,
    teacher_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    start_date DATE,
    end_date DATE,
    subject_area VARCHAR(100),
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (teacher_id) REFERENCES teacher(teacher_id) ON DELETE CASCADE
);

-- Create ENROLLMENT table
CREATE TABLE enrollment (
    enrollment_id INT PRIMARY KEY AUTO_INCREMENT,
    student_id INT NOT NULL,
    course_id INT NOT NULL,
    enrollment_date DATE DEFAULT (CURRENT_DATE),
    status VARCHAR(20) DEFAULT 'active',
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (student_id) REFERENCES student(student_id) ON DELETE CASCADE,
    FOREIGN KEY (course_id) REFERENCES classroom(course_id) ON DELETE CASCADE
);

-- Create MATERIAL table
CREATE TABLE material (
    material_id INT PRIMARY KEY AUTO_INCREMENT,
    course_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50),
    file_path VARCHAR(255),
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    description TEXT,
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (course_id) REFERENCES classroom(course_id) ON DELETE CASCADE
);

-- Create ANNOUNCEMENT table
CREATE TABLE announcement (
    announcement_id INT PRIMARY KEY AUTO_INCREMENT,
    course_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_pinned BOOLEAN DEFAULT FALSE,
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (course_id) REFERENCES classroom(course_id) ON DELETE CASCADE
);

-- Create ASSIGNMENT table
CREATE TABLE assignment (
    assignment_id INT PRIMARY KEY AUTO_INCREMENT,
    course_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    due_date DATETIME NOT NULL,
    max_points INT DEFAULT 100,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (course_id) REFERENCES classroom(course_id) ON DELETE CASCADE
);

-- Create SUBMISSION table
CREATE TABLE submission (
    submission_id INT PRIMARY KEY AUTO_INCREMENT,
    assignment_id INT NOT NULL,
    student_id INT NOT NULL,
    content VARCHAR(255),
    submitted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    score INT,
    feedback TEXT,
    status VARCHAR(20) DEFAULT 'submitted',
    archive_delete_flag BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (assignment_id) REFERENCES assignment(assignment_id) ON DELETE CASCADE,
    FOREIGN KEY (student_id) REFERENCES student(student_id) ON DELETE CASCADE
);

-- Create indexes for performance optimization
CREATE INDEX idx_user_email ON user(email);
CREATE INDEX idx_student_user ON student(user_id);
CREATE INDEX idx_teacher_user ON teacher(user_id);
CREATE INDEX idx_classroom_teacher ON classroom(teacher_id);
CREATE INDEX idx_enrollment_student ON enrollment(student_id);
CREATE INDEX idx_enrollment_course ON enrollment(course_id);
CREATE INDEX idx_material_course ON material(course_id);
CREATE INDEX idx_announcement_course ON announcement(course_id);
CREATE INDEX idx_assignment_course ON assignment(course_id);
CREATE INDEX idx_submission_assignment ON submission(assignment_id);
CREATE INDEX idx_submission_student ON submission(student_id);

-- Add unique constraint to prevent duplicate enrollments
ALTER TABLE enrollment ADD CONSTRAINT uq_student_course UNIQUE (student_id, course_id);

